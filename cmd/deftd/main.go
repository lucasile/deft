package main

import (
	"context"
	"os"

	"github.com/lucasile/deft/internal/agent/config"
	"github.com/lucasile/deft/internal/agent/docker"
	"github.com/lucasile/deft/internal/agent/tunnel"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("Deft Daemon starting...")

	cfg, err := config.Load("/etc/deft/agent.json")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config from /etc/deft/agent.json")
	}

	dockerClient, err := docker.NewClient()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to docker")
	}
	defer dockerClient.Close()

	ctx := context.Background()
	conn, err := tunnel.NewConnection(ctx, cfg.PanelAddr, cfg.CAPath, cfg.CertPath, cfg.KeyPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create tunnel connection")
	}

	if err := conn.Connect(ctx, cfg.NodeID); err != nil {
		log.Fatal().Err(err).Msg("failed to connect to panel")
	}

	handler := tunnel.NewHandler(dockerClient, conn, cfg.NodeID)

	for {
		cmd, err := conn.Receive()
		if err != nil {
			log.Error().Err(err).Msg("connection lost, reconnecting...")
			if err := conn.Connect(ctx, cfg.NodeID); err != nil {
				log.Fatal().Err(err).Msg("failed to reconnect")
			}
			continue
		}

		go handler.HandleCommand(ctx, cmd)
	}
}

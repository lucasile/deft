package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/lucasile/deft/internal/agent/config"
	"github.com/lucasile/deft/internal/agent/docker"
	"github.com/lucasile/deft/internal/agent/tunnel"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	defaultConfigPath := os.Getenv("DEFT_AGENT_CONFIG")
	if defaultConfigPath == "" {
		defaultConfigPath = "/etc/deft/agent.json"
	}
	configPath := flag.String("config", defaultConfigPath, "path to agent config JSON")
	flag.Parse()

	log.Info().Msg("Deft Daemon starting...")
	log.Info().Str("path", *configPath).Msg("loading agent config")

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", *configPath).Msg("failed to load agent config")
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
	reconnectBackoff := 1 * time.Second
	maxReconnectBackoff := 60 * time.Second

	for {
		cmd, err := conn.Receive()
		if err != nil {
			log.Error().Err(err).Dur("retry_in", reconnectBackoff).Msg("connection lost, reconnecting")
			time.Sleep(reconnectBackoff)
			reconnectBackoff *= 2
			if reconnectBackoff > maxReconnectBackoff {
				reconnectBackoff = maxReconnectBackoff
			}
			if err := conn.Connect(ctx, cfg.NodeID); err != nil {
				log.Fatal().Err(err).Msg("failed to reconnect")
			}
			continue
		}

		reconnectBackoff = 1 * time.Second
		go handler.HandleCommand(ctx, cmd)
	}
}

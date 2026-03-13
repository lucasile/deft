package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/lucasile/deft/agent/config"
	"github.com/lucasile/deft/agent/docker"
	"github.com/lucasile/deft/agent/tunnel"
	"github.com/lucasile/deft/proto"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:           "serve",
	Short:         "Start the Deft agent service",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	Elevate()

	log.Info().Msg("Deft agent starting...")

	cfg, err := config.Load("/etc/deft/agent.json")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load config from /etc/deft/agent.json, using defaults or env")
		// For now, allow it to continue with some defaults if config missing, or just fail
		// But let's fail for now to be explicit
		return fmt.Errorf("config error: %w", err)
	}

	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("docker error: %w", err)
	}
	defer dockerClient.Close()

	ctx := context.Background()
	conn, err := tunnel.NewConnection(ctx, cfg.PanelAddr, cfg.CAPath, cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return fmt.Errorf("tunnel error: %w", err)
	}

	if err := conn.Connect(ctx, cfg.NodeID); err != nil {
		return fmt.Errorf("failed to establish tunnel: %w", err)
	}

	handler := tunnel.NewHandler(dockerClient, conn, cfg.NodeID)

	log.Info().Msg("Agent is ready and listening for commands")

	// Heartbeat loop
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := conn.SendHeartbeat(cfg.NodeID); err != nil {
					log.Error().Err(err).Msg("Failed to send heartbeat")
				}
			}
		}
	}()

	// Command receiving loop
	for {
		cmd, err := conn.Receive()
		if err != nil {
			log.Error().Err(err).Msg("Lost connection to panel, attempting to reconnect...")
			if err := conn.Connect(ctx, cfg.NodeID); err != nil {
				log.Fatal().Err(err).Msg("Failed to reconnect to panel")
			}
			continue
		}

		log.Info().Str("command_id", cmd.CommandId).Msg("Received command from panel")
		
		// If command is 'start', we should also start a stats streamer for that container
		// For now, the handler handles the Docker logic, but we could add stats logic here
		// or inside the handler. Let's keep it simple for MVP.

		go func(c *tunnel.Connection, command *proto.PanelCommand) {
			if err := handler.HandleCommand(ctx, command); err != nil {
				log.Error().Err(err).Str("command_id", command.CommandId).Msg("Failed to handle command")
			}
		}(conn, cmd)
	}
}

package cmd

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Deft agent service",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	log.Info().Msg("Deft agent starting...")
	log.Info().Msg("Phase 1 MVP: Serve command placeholder active.")

	// For now, just keep the process alive
	for {
		log.Debug().Msg("Agent heartbeat...")
		time.Sleep(30 * time.Second)
	}
}

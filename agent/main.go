package main

import (
	"github.com/lucasile/deft/agent/cmd"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	if err := cmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("failed to execute command")
	}
}

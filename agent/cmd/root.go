package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "deft",
	Short: "Deft Server Manager",
	Long:  "A lightweight agent that manages game servers and general Docker applications.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Root command initialization logic
}

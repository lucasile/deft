package cmd

import (
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the Deft agent service",
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Deft agent service",
	RunE: func(cmd *cobra.Command, args []string) error {
		Elevate()
		return runCommand("systemctl", "start", "deft")
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Deft agent service",
	RunE: func(cmd *cobra.Command, args []string) error {
		Elevate()
		return runCommand("systemctl", "stop", "deft")
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the Deft agent service",
	RunE: func(cmd *cobra.Command, args []string) error {
		Elevate()
		return runCommand("systemctl", "restart", "deft")
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the Deft agent service",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommand("systemctl", "status", "deft")
	},
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(startCmd)
	serviceCmd.AddCommand(stopCmd)
	serviceCmd.AddCommand(restartCmd)
	serviceCmd.AddCommand(statusCmd)
}

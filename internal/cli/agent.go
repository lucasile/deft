package cli

import (
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage the Deft agent daemon",
}

var agentStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Deft agent daemon service",
	RunE: func(cmd *cobra.Command, args []string) error {
		Elevate()
		return runCommand("systemctl", "start", "deft")
	},
}

var agentStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Deft agent daemon service",
	RunE: func(cmd *cobra.Command, args []string) error {
		Elevate()
		return runCommand("systemctl", "stop", "deft")
	},
}

var agentRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the Deft agent daemon service",
	RunE: func(cmd *cobra.Command, args []string) error {
		Elevate()
		return runCommand("systemctl", "restart", "deft")
	},
}

var agentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of the Deft agent daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = runCommand("systemctl", "status", "deft")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentStartCmd)
	agentCmd.AddCommand(agentStopCmd)
	agentCmd.AddCommand(agentRestartCmd)
	agentCmd.AddCommand(agentStatusCmd)
}

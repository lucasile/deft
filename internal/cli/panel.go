package cli

import (
	"github.com/spf13/cobra"
)

var panelCmd = &cobra.Command{
	Use:   "panel",
	Short: "Manage the Deft panel container",
}

var panelStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Deft panel container",
	RunE: func(cmd *cobra.Command, args []string) error {
		Elevate()
		return runCommand("docker", "start", "deft-panel")
	},
}

var panelStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Deft panel container",
	RunE: func(cmd *cobra.Command, args []string) error {
		Elevate()
		return runCommand("docker", "stop", "deft-panel")
	},
}

var panelLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show logs of the Deft panel container",
	RunE: func(cmd *cobra.Command, args []string) error {
		Elevate()
		return runCommand("docker", "logs", "-f", "deft-panel")
	},
}

func init() {
	rootCmd.AddCommand(panelCmd)
	panelCmd.AddCommand(panelStartCmd)
	panelCmd.AddCommand(panelStopCmd)
	panelCmd.AddCommand(panelLogsCmd)
}

package cli

import (
	"fmt"
	"strings"

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

var panelRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the Deft panel container",
	RunE: func(cmd *cobra.Command, args []string) error {
		Elevate()
		return runCommand("docker", "restart", "deft-panel")
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

var panelConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Reconfigure the Deft panel container (UI port)",
	RunE: func(cmd *cobra.Command, args []string) error {
		Elevate()
		httpPort, _ := cmd.Flags().GetString("port")
		httpPort = strings.TrimSpace(httpPort)

		fmt.Printf("Updating panel configuration: UI=%s\n", httpPort)

		// Stop and remove existing
		_ = runCommandQuiet("docker", "stop", "deft-panel")
		_ = runCommandQuiet("docker", "rm", "deft-panel")

		// Recreate with new UI port, keep gRPC at 50051
		image := "ghcr.io/lucasile/deft-panel:latest"
		return runCommand("docker", "run", "-d",
			"--name", "deft-panel",
			"--restart", "always",
			"-p", httpPort+":3000",
			"-p", "50051:50051",
			"-v", "/etc/deft:/etc/deft:ro",
			"-v", "deft-panel-data:/data",
			image)
	},
}

func init() {
	rootCmd.AddCommand(panelCmd)
	panelCmd.AddCommand(panelStartCmd)
	panelCmd.AddCommand(panelStopCmd)
	panelCmd.AddCommand(panelRestartCmd)
	panelCmd.AddCommand(panelLogsCmd)
	panelCmd.AddCommand(panelConfigCmd)

	panelConfigCmd.Flags().String("port", "3000", "Panel UI Port")
}

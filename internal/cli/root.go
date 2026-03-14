package cli

import (
	"fmt"
	"os"

	"github.com/lucasile/deft/internal/i18n"
	"github.com/manifoldco/promptui"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "deft",
	Short:         "Deft Universal Controller",
	Long:          "A unified CLI to manage the Deft Agent daemon and the Panel container.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Deft components and clean up",
	RunE:  runUninstall,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	Elevate()

	langSelect := promptui.Select{
		Label: "Select Language / Selectați limba",
		Items: []string{"English (en)", "Romanian (ro)"},
	}
	idx, _, err := langSelect.Run()
	if err != nil {
		return err
	}
	lang := "en"
	if idx == 1 {
		lang = "ro"
	}
	i18n.Init(lang)

	log.Info().Msg(i18n.T("UninstallStart", nil))

	// 1. Agent Cleanup
	_ = runCommandQuiet("systemctl", "stop", "deft")
	_ = runCommandQuiet("systemctl", "disable", "deft")
	_ = runCommandQuiet("systemctl", "unmask", "deft")

	serviceFile := "/etc/systemd/system/deft.service"
	if _, err := os.Lstat(serviceFile); err == nil {
		if err := os.Remove(serviceFile); err != nil {
			log.Warn().Err(err).Msg("Failed to remove service file")
		} else {
			log.Info().Msg(i18n.T("ServiceRemoved", nil))
		}
	}

	_ = runCommandQuiet("systemctl", "daemon-reload")
	log.Info().Msg(i18n.T("DaemonReloaded", nil))

	// 2. Panel Cleanup (Docker)
	if confirm(i18n.T("ConfirmPanelRemoval", nil)) {
		_ = runCommandQuiet("docker", "stop", "deft-panel")
		_ = runCommandQuiet("docker", "rm", "deft-panel")
		log.Info().Msg(i18n.T("PanelRemoved", nil))

		if confirm(i18n.T("ConfirmPanelDataRemoval", nil)) {
			_ = runCommandQuiet("docker", "volume", "rm", "deft-panel-data")
			log.Info().Msg(i18n.T("PanelDataRemoved", nil))
		}
	}

	// 3. Binary Cleanup
	binaryPath := "/usr/local/bin/deft"
	if _, err := os.Lstat(binaryPath); err == nil {
		if err := os.Remove(binaryPath); err != nil {
			log.Warn().Err(err).Msg("Failed to remove binary")
		} else {
			log.Info().Msg(i18n.T("BinaryRemoved", map[string]interface{}{"Path": binaryPath}))
		}
	}

	daemonPath := "/usr/local/bin/deftd"
	if _, err := os.Lstat(daemonPath); err == nil {
		_ = os.Remove(daemonPath)
	}

	// 4. Directory Cleanup
	if confirm(i18n.T("ConfirmConfigRemoval", nil)) {
		if err := os.RemoveAll("/etc/deft"); err != nil {
			log.Warn().Err(err).Msg("Failed to remove /etc/deft")
		} else {
			log.Info().Msg(i18n.T("DirRemoved", map[string]interface{}{"Path": "/etc/deft"}))
		}
	}

	if confirm(i18n.T("ConfirmDataRemoval", nil)) {
		if err := os.RemoveAll("/var/lib/deft/volumes"); err != nil {
			log.Warn().Err(err).Msg("Failed to remove /var/lib/deft/volumes")
		} else {
			log.Info().Msg(i18n.T("DirRemoved", map[string]interface{}{"Path": "/var/lib/deft/volumes"}))
		}
	}

	fmt.Printf("\n%s\n", i18n.T("SuccessUninstall", nil))
	return nil
}

func confirm(label string) bool {
	prompt := promptui.Prompt{
		Label:     label,
		IsConfirm: true,
	}
	result, err := prompt.Run()
	if err != nil {
		return false
	}
	return result == "y" || result == "Y"
}

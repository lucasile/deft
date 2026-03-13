package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/lucasile/deft/internal/i18n"
	"github.com/manifoldco/promptui"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the Deft agent and clean up",
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	if os.Geteuid() != 0 {
		fmt.Println("This command must be run as root (or with sudo).")
		fmt.Println("Attempting to elevate permissions with sudo...")

		executable, err := os.Executable()
		if err != nil {
			return err
		}

		args := append([]string{executable}, os.Args[1:]...)
		cmd := exec.Command("sudo", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err = cmd.Run(); err != nil {
			return fmt.Errorf("elevation failed: %w", err)
		}
		return nil
	}

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

	_ = runCommand("systemctl", "stop", "deft")
	_ = runCommand("systemctl", "disable", "deft")
	_ = runCommand("systemctl", "unmask", "deft")

	serviceFile := "/etc/systemd/system/deft.service"
	if _, err := os.Lstat(serviceFile); err == nil {
		if err := os.Remove(serviceFile); err != nil {
			log.Warn().Err(err).Msg("Failed to remove service file")
		} else {
			log.Info().Msg(i18n.T("ServiceRemoved", nil))
		}
	}

	_ = runCommand("systemctl", "daemon-reload")
	log.Info().Msg(i18n.T("DaemonReloaded", nil))

	binaryPath := "/usr/local/bin/deft"
	if _, err := os.Lstat(binaryPath); err == nil {
		if err := os.Remove(binaryPath); err != nil {
			log.Warn().Err(err).Msg("Failed to remove binary")
		} else {
			log.Info().Msg(i18n.T("BinaryRemoved", map[string]interface{}{"Path": binaryPath}))
		}
	}

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

package main

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/deft/internal/i18n"
	"github.com/manifoldco/promptui"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

//go:embed templates/deft.service
var serviceTemplate []byte

func main() {
	if os.Geteuid() != 0 {
		fmt.Println("This command must be run as root (or with sudo).")
		fmt.Println("Attempting to elevate permissions with sudo...")
		
		executable, err := os.Executable()
		if err != nil {
			fmt.Printf("Failed to get executable path: %v\n", err)
			os.Exit(1)
		}

		args := append([]string{executable}, os.Args[1:]...)
		cmd := exec.Command("sudo", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err = cmd.Run(); err != nil {
			fmt.Printf("Elevation failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	langSelect := promptui.Select{
		Label: "Select Language / Selectați limba",
		Items: []string{"English (en)", "Romanian (ro)"},
	}
	idx, _, err := langSelect.Run()
	if err != nil {
		fmt.Printf("Selection failed: %v\n", err)
		os.Exit(1)
	}
	lang := "en"
	if idx == 1 {
		lang = "ro"
	}
	i18n.Init(lang)

	modeSelect := promptui.Select{
		Label: i18n.T("ModeSelectLabel", nil),
		Items: []string{i18n.T("ModeDefault", nil), i18n.T("ModeVerbose", nil)},
	}
	mIdx, _, err := modeSelect.Run()
	if err != nil {
		fmt.Println(i18n.T("SelectionFailed", map[string]interface{}{"Err": err}))
		os.Exit(1)
	}
	if mIdx == 1 {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg(i18n.T("InstallStart", nil))

	if err := checkDocker(); err != nil {
		log.Fatal().Msg(i18n.T("DockerCheckFailed", map[string]interface{}{"Err": err}))
	}

	osName := runtime.GOOS
	arch := runtime.GOARCH
	log.Info().Msg(i18n.T("DownloadingBinary", map[string]interface{}{"OS": osName, "Arch": arch}))

	downloadURL := fmt.Sprintf(agentDownloadURLFormat, osName, arch)
	tempFile := "deft.tmp"
	
	if err = downloadFile(tempFile, downloadURL); err != nil {
		log.Warn().Err(err).Msg(i18n.T("DownloadFailedRemote", nil))
		log.Info().Msg(i18n.T("CheckingLocal", nil))
		
		if _, err := os.Stat("./bin/deft"); err == nil {
			log.Info().Msg(i18n.T("FoundLocal", nil))
			if err := copyFile("./bin/deft", tempFile); err != nil {
				log.Fatal().Err(err).Msg("failed to copy local binary")
			}
		} else {
			log.Fatal().Msg(i18n.T("NoBinaryFound", nil))
		}
	}

	if err := copyFile(tempFile, defaultBinaryPath); err != nil {
		log.Fatal().Err(err).Msgf("failed to copy binary to %s", defaultBinaryPath)
	}
	if err := os.Chmod(defaultBinaryPath, 0755); err != nil {
		log.Fatal().Err(err).Msgf("failed to set permissions on %s", defaultBinaryPath)
	}
	_ = os.Remove(tempFile)
	log.Info().Msg(i18n.T("BinaryInstalled", map[string]interface{}{"Path": defaultBinaryPath}))

	dirs := []string{configDirPath, volumesDirPath}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatal().Err(err).Msgf("failed to create directory %s", dir)
		}
		log.Info().Msg(i18n.T("DirCreated", map[string]interface{}{"Path": dir}))
	}

	if err := os.WriteFile(servicePath, serviceTemplate, 0644); err != nil {
		log.Fatal().Err(err).Msg("failed to write systemd service file")
	}
	log.Info().Str("path", servicePath).Msg("Systemd service file created")

	log.Info().Msg(i18n.T("ReloadingSystemd", nil))
	_ = runCommand("systemctl", "daemon-reload")

	log.Info().Msg(i18n.T("EnablingService", nil))
	_ = runCommand("systemctl", "enable", "deft")

	log.Info().Msg(i18n.T("StartingService", nil))
	_ = runCommand("systemctl", "start", "deft")

	fmt.Printf("\n%s\n", i18n.T("Success", nil))
	fmt.Println(i18n.T("NextSteps", nil))
	fmt.Println(i18n.T("ConfigHint", nil))
	fmt.Println(i18n.T("LogHint", nil))
}

func checkDocker() error {
	_, err := exec.LookPath("docker")
	if err == nil {
		log.Info().Msg("Docker is already installed")
		return nil
	}
	log.Warn().Msg(i18n.T("DockerNotFound", nil))
	return fmt.Errorf(i18n.T("DockerNotFound", nil))
}

func downloadFile(filepath string, url string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}
	_, err = io.Copy(out, resp.Body)
	return err
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	_ = os.Remove(dst)
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()
	_, err = io.Copy(destFile, sourceFile)
	return err
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

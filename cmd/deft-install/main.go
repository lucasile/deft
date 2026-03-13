package main

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/lucasile/deft/internal/i18n"
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

	compSelect := promptui.Select{
		Label: i18n.T("ComponentSelectLabel", nil),
		Items: []string{
			i18n.T("CompAgentOnly", nil),
			i18n.T("CompPanelOnly", nil),
			i18n.T("CompBoth", nil),
		},
	}
	cIdx, _, err := compSelect.Run()
	if err != nil {
		log.Fatal().Err(err).Msg("component selection failed")
	}

	installAgent := cIdx == 0 || cIdx == 2
	installPanel := cIdx == 1 || cIdx == 2

	if err := checkDocker(); err != nil {
		log.Fatal().Msg(i18n.T("DockerCheckFailed", map[string]interface{}{"Err": err}))
	}

	if installAgent {
		runAgentInstall()
	}

	if installPanel {
		httpPort := promptDefault("Enter Panel Web Port", "3000")
		grpcPort := promptDefault("Enter Agent gRPC Port", "50051")
		runPanelInstall(httpPort, grpcPort)
	}

	fmt.Printf("\n%s\n", i18n.T("Success", nil))
}

func promptDefault(label, defaultValue string) string {
	prompt := promptui.Prompt{
		Label:   label,
		Default: defaultValue,
	}
	result, err := prompt.Run()
	if err != nil {
		return defaultValue
	}
	return result
}

func runAgentInstall() {
	log.Info().Msg(i18n.T("InstallStart", nil))
	
	installBinary("deft", defaultBinaryPath)
	
	daemonPath := "/usr/local/bin/deftd"
	installBinary("deftd", daemonPath)

	dirs := []string{configDirPath, volumesDirPath}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatal().Err(err).Msgf("failed to create directory %s", dir)
		}
		log.Info().Msg(i18n.T("DirCreated", map[string]interface{}{"Path": dir}))
	}

	_ = runCommandQuiet("systemctl", "stop", "deft")
	_ = runCommandQuiet("systemctl", "disable", "deft")
	_ = runCommandQuiet("systemctl", "unmask", "deft")
	_ = os.Remove(servicePath)
	_ = runCommandQuiet("systemctl", "daemon-reload")

	if err := os.WriteFile(servicePath, serviceTemplate, 0644); err != nil {
		log.Fatal().Err(err).Msg("failed to write systemd service file")
	}
	log.Info().Str("path", servicePath).Msg("Systemd service file created")

	log.Info().Msg(i18n.T("ReloadingSystemd", nil))
	_ = runCommandQuiet("systemctl", "daemon-reload")

	log.Info().Msg(i18n.T("EnablingService", nil))
	_ = runCommand("systemctl", "enable", "deft")

	log.Info().Msg(i18n.T("StartingService", nil))
	_ = runCommand("systemctl", "start", "deft")
}

func runPanelInstall(httpPort, grpcPort string) {
	log.Info().Msg("Installing Deft Panel via Docker...")
	
	image := "ghcr.io/lucasile/deft-panel:latest"
	
	log.Info().Str("image", image).Msg("Pulling panel image...")
	if err := runCommand("docker", "pull", image); err != nil {
		log.Warn().Err(err).Msg("Failed to pull remote image, checking for local image...")
	}

	err := runCommand("docker", "run", "-d",
		"--name", "deft-panel",
		"--restart", "always",
		"-p", httpPort+":3000",
		"-p", grpcPort+":50051",
		"-v", "deft-panel-data:/data",
		image)
	
	if err != nil {
		log.Error().Err(err).Msg("Failed to start panel container")
	} else {
		log.Info().Msgf("Panel container started on ports %s (UI) and %s (gRPC)", httpPort, grpcPort)
	}
}

func installBinary(name, target string) {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	tempFile := name + ".tmp"
	downloadURL := fmt.Sprintf(agentDownloadURLFormat, name, osName, arch)

	if err := downloadFile(tempFile, downloadURL); err != nil {
		log.Warn().Err(err).Msgf("Could not download %s, checking ./bin/%s", name, name)
		localPath := "./bin/" + name
		if _, err := os.Stat(localPath); err == nil {
			if err := copyFile(localPath, tempFile); err != nil {
				log.Fatal().Err(err).Msgf("failed to copy local %s", name)
			}
		} else {
			log.Fatal().Msgf("No %s binary found", name)
		}
	}

	if err := copyFile(tempFile, target); err != nil {
		log.Fatal().Err(err).Msgf("failed to copy %s to %s", name, target)
	}
	if err := os.Chmod(target, 0755); err != nil {
		log.Fatal().Err(err).Msgf("failed to set permissions on %s", target)
	}
	_ = os.Remove(tempFile)
	log.Info().Msg(i18n.T("BinaryInstalled", map[string]interface{}{"Path": target}))
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

func runCommandQuiet(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

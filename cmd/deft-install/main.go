package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/lucasile/deft/internal/i18n"
	"github.com/manifoldco/promptui"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

//go:embed templates/deft.service
var serviceTemplate []byte

var stdinReader = bufio.NewReader(os.Stdin)

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
		httpPort := strings.TrimSpace(promptDefault("Enter Panel Web Port", "3000"))
		runPanelInstall(httpPort, "50051")
	}

	fmt.Printf("\n%s\n", i18n.T("Success", nil))
}

func promptDefault(label, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", label, defaultValue)
	} else {
		fmt.Printf("%s: ", label)
	}
	result, err := stdinReader.ReadString('\n')
	if err != nil {
		return defaultValue
	}
	result = strings.TrimSpace(result)
	if result == "" {
		return defaultValue
	}
	return result
}

func promptYesNo(label string, defaultValue bool) bool {
	defaultLabel := "n"
	if defaultValue {
		defaultLabel = "y"
	}
	result := strings.ToLower(strings.TrimSpace(promptDefault(label+" (y/n)", defaultLabel)))
	return result == "y" || result == "yes"
}

func runAgentInstall() {
	log.Info().Msg(i18n.T("InstallStart", nil))

	installBinary("deft", defaultBinaryPath)

	daemonPath := "/usr/local/bin/deftd"
	installBinary("deftd", daemonPath)

	dirs := []string{configDirPath, certsDirPath, volumesDirPath}
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

	if promptYesNo("Join this agent to a panel now?", true) {
		for {
			panelURL := strings.TrimRight(strings.TrimSpace(promptDefault("Panel URL", "https://panel.example.com")), "/")
			nodeName := strings.TrimSpace(promptDefault("Node name", ""))
			if err := runAgentJoin(panelURL, nodeName); err != nil {
				log.Error().Err(err).Msg("failed to join panel")
				if promptYesNo("Retry agent join?", true) {
					continue
				}
			}
			break
		}
	}

	if _, err := os.Stat(agentConfigPath); err != nil {
		log.Warn().Str("path", agentConfigPath).Msg("agent config missing; not starting service")
		return
	}

	log.Info().Msg(i18n.T("StartingService", nil))
	_ = runCommand("systemctl", "restart", "deft")
}

func runPanelInstall(httpPort, grpcPort string) {
	log.Info().Msg("Installing Deft Panel via Docker...")

	image := "ghcr.io/lucasile/deft-panel:latest"

	log.Info().Str("image", image).Msg("Pulling panel image...")
	if err := runCommand("docker", "pull", image); err != nil {
		log.Warn().Err(err).Msg("Failed to pull remote image, checking for local image...")
	}

	// Cleanup existing panel if it exists
	_ = runCommandQuiet("docker", "stop", "deft-panel")
	_ = runCommandQuiet("docker", "rm", "deft-panel")

	err := runCommand("docker", "run", "-d",
		"--name", "deft-panel",
		"--restart", "always",
		"-p", httpPort+":3000",
		"-p", grpcPort+":50051",
		"-v", "/etc/deft:/etc/deft:ro",
		"-v", "deft-panel-data:/data",
		image)

	if err != nil {
		log.Error().Err(err).Msg("Failed to start panel container")
	} else {
		log.Info().Msgf("Panel container started on ports %s (UI) and %s (gRPC)", httpPort, grpcPort)
	}
}

type agentJoinRequest struct {
	NodeName string `json:"node_name"`
	CSRPem   string `json:"csr_pem"`
}

type createJoinRequestRequest struct {
	NodeName string `json:"node_name"`
	CSRPem   string `json:"csr_pem"`
	PanelURL string `json:"panel_url"`
}

type joinRequestResponse struct {
	ID               string `json:"id"`
	Secret           string `json:"secret"`
	VerificationCode string `json:"verification_code"`
	ApprovalURL      string `json:"approval_url"`
	ExpiresAt        int64  `json:"expires_at"`
}

type joinRequestStatusResponse struct {
	Status string             `json:"status"`
	Result *agentJoinResponse `json:"result"`
}

type agentJoinResponse struct {
	NodeID    string `json:"node_id"`
	PanelAddr string `json:"panel_addr"`
	CACertPEM string `json:"ca_cert_pem"`
	CertPEM   string `json:"cert_pem"`
}

type agentConfig struct {
	PanelAddr string `json:"panel_addr"`
	NodeID    string `json:"node_id"`
	CAPath    string `json:"ca_path"`
	CertPath  string `json:"cert_path"`
	KeyPath   string `json:"key_path"`
}

func runAgentJoin(panelURL, nodeName string) error {
	methodSelect := promptui.Select{
		Label: "Agent join method",
		Items: []string{
			"Browser approval link",
			"Join token",
		},
	}
	idx, _, err := methodSelect.Run()
	if err != nil {
		return fmt.Errorf("join method selection failed: %w", err)
	}

	if idx == 0 {
		return joinPanelWithApproval(panelURL, nodeName)
	}

	joinToken := strings.TrimSpace(promptDefault("Join token", ""))
	return joinPanelWithToken(panelURL, joinToken, nodeName)
}

func joinPanel(panelURL, joinToken, nodeName string) error {
	return joinPanelWithToken(panelURL, joinToken, nodeName)
}

func joinPanelWithToken(panelURL, joinToken, nodeName string) error {
	if panelURL == "" {
		return fmt.Errorf("panel URL is required")
	}
	if joinToken == "" {
		return fmt.Errorf("join token is required")
	}
	if err := validateJoinTokenSecret(joinToken); err != nil {
		return err
	}

	keyPEM, csrPEM, err := createAgentKeyAndCSR()
	if err != nil {
		return err
	}

	joinResult, err := requestAgentJoin(panelURL, joinToken, nodeName, csrPEM)
	if err != nil {
		return err
	}

	if err := writeAgentJoinFiles(joinResult, keyPEM); err != nil {
		return err
	}

	log.Info().Str("node_id", joinResult.NodeID).Msg("Agent joined panel")
	return nil
}

func validateJoinTokenSecret(token string) error {
	if len(token) != 64 {
		return fmt.Errorf("join token must be 64 hex characters; copy the full token shown after creation, not the recent-token id")
	}
	if _, err := hex.DecodeString(token); err != nil {
		return fmt.Errorf("join token must be hex: %w", err)
	}
	return nil
}

func joinPanelWithApproval(panelURL, nodeName string) error {
	if panelURL == "" {
		return fmt.Errorf("panel URL is required")
	}

	keyPEM, csrPEM, err := createAgentKeyAndCSR()
	if err != nil {
		return err
	}

	joinRequest, err := createJoinRequest(panelURL, nodeName, csrPEM)
	if err != nil {
		return err
	}

	fmt.Printf("\nOpen this URL in your browser and approve the node:\n\n%s\n\nVerification code: %s\n\n", joinRequest.ApprovalURL, joinRequest.VerificationCode)

	joinResult, err := waitForJoinApproval(panelURL, joinRequest.ID, joinRequest.Secret, joinRequest.ExpiresAt)
	if err != nil {
		return err
	}

	if err := writeAgentJoinFiles(joinResult, keyPEM); err != nil {
		return err
	}

	log.Info().Str("node_id", joinResult.NodeID).Msg("Agent joined panel")
	return nil
}

func createAgentKeyAndCSR() ([]byte, string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate agent private key: %w", err)
	}

	csrTemplate := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "deft-agent"},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, key)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create agent CSR: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if keyPEM == nil {
		return nil, "", fmt.Errorf("failed to encode agent private key")
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	})
	if csrPEM == nil {
		return nil, "", fmt.Errorf("failed to encode agent CSR")
	}

	return keyPEM, string(csrPEM), nil
}

func requestAgentJoin(panelURL, joinToken, nodeName, csrPEM string) (*agentJoinResponse, error) {
	body, err := json.Marshal(agentJoinRequest{
		NodeName: nodeName,
		CSRPem:   csrPEM,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode join request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, panelURL+"/api/agent/join", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create join request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+joinToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call join endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("join failed: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}

	var result agentJoinResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode join response: %w", err)
	}
	if result.NodeID == "" || result.PanelAddr == "" || result.CACertPEM == "" || result.CertPEM == "" {
		return nil, fmt.Errorf("join response missing required fields")
	}

	return &result, nil
}

func createJoinRequest(panelURL, nodeName, csrPEM string) (*joinRequestResponse, error) {
	body, err := json.Marshal(createJoinRequestRequest{
		NodeName: nodeName,
		CSRPem:   csrPEM,
		PanelURL: panelURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode join request: %w", err)
	}

	resp, err := http.Post(panelURL+"/api/agent/join-requests", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create join request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("join request failed: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}

	var result joinRequestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode join request response: %w", err)
	}
	if result.ID == "" || result.Secret == "" || result.ApprovalURL == "" || result.VerificationCode == "" {
		return nil, fmt.Errorf("join request response missing required fields")
	}

	return &result, nil
}

func waitForJoinApproval(panelURL, requestID, secret string, expiresAt int64) (*agentJoinResponse, error) {
	for time.Now().Unix() < expiresAt {
		result, err := joinRequestStatus(panelURL, requestID, secret)
		if err != nil {
			return nil, err
		}

		switch result.Status {
		case "approved":
			if result.Result == nil {
				return nil, fmt.Errorf("approved join request missing result")
			}
			return result.Result, nil
		case "pending":
			time.Sleep(3 * time.Second)
		case "denied":
			return nil, fmt.Errorf("join request denied")
		case "expired":
			return nil, fmt.Errorf("join request expired")
		default:
			return nil, fmt.Errorf("unknown join request status: %s", result.Status)
		}
	}

	return nil, fmt.Errorf("join request expired")
}

func joinRequestStatus(panelURL, requestID, secret string) (*joinRequestStatusResponse, error) {
	req, err := http.NewRequest(http.MethodGet, panelURL+"/api/agent/join-requests/"+requestID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create join status request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+secret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to check join status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("join status failed: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}

	var result joinRequestStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode join status response: %w", err)
	}
	return &result, nil
}

func writeAgentJoinFiles(result *agentJoinResponse, keyPEM []byte) error {
	if err := os.MkdirAll(certsDirPath, 0755); err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	caPath := certsDirPath + "/ca.crt"
	certPath := certsDirPath + "/agent.crt"
	keyPath := certsDirPath + "/agent.key"

	if err := os.WriteFile(caPath, []byte(result.CACertPEM), 0644); err != nil {
		return fmt.Errorf("failed to write CA cert: %w", err)
	}
	if err := os.WriteFile(certPath, []byte(result.CertPEM), 0644); err != nil {
		return fmt.Errorf("failed to write agent cert: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write agent key: %w", err)
	}

	configData, err := json.MarshalIndent(agentConfig{
		PanelAddr: result.PanelAddr,
		NodeID:    result.NodeID,
		CAPath:    caPath,
		CertPath:  certPath,
		KeyPath:   keyPath,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode agent config: %w", err)
	}
	configData = append(configData, '\n')

	if err := os.WriteFile(agentConfigPath, configData, 0600); err != nil {
		return fmt.Errorf("failed to write agent config: %w", err)
	}

	return nil
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
	return errors.New(i18n.T("DockerNotFound", nil))
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

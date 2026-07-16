package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

const maxJSONBodyBytes = 32 * 1024

var (
	containerNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)
	nodeNamePattern      = regexp.MustCompile(`^[^\x00-\x1f\x7f]{1,64}$`)
	containerIDPattern   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:-]{0,127}$`)
	commandIDPattern     = regexp.MustCompile(`^[a-f0-9]{32}$`)
	envKeyPattern        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)
	joinTokenIDPattern   = regexp.MustCompile(`^[a-f0-9]{32}$`)
	joinRequestIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
	imagePattern         = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{0,254}$`)
)

func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("invalid json body: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("invalid json body: multiple json values")
	}

	return nil
}

func validateNodeID(value string) error {
	if !containerIDPattern.MatchString(value) {
		return fmt.Errorf("invalid node id")
	}
	return nil
}

func validateNodeName(value string) error {
	if !nodeNamePattern.MatchString(value) {
		return fmt.Errorf("node name must be 1-64 characters and cannot contain control characters")
	}
	return nil
}

func validateContainerName(value string) error {
	if !containerNamePattern.MatchString(value) {
		return fmt.Errorf("container name must be 1-64 characters and use only letters, numbers, dots, underscores, or dashes")
	}
	return nil
}

func validateContainerID(value string) error {
	if !containerIDPattern.MatchString(value) {
		return fmt.Errorf("invalid container id")
	}
	return nil
}

func validateCommandID(value string) error {
	if !commandIDPattern.MatchString(value) {
		return fmt.Errorf("invalid command id")
	}
	return nil
}

func validateJoinRequestID(value string) error {
	if !joinRequestIDPattern.MatchString(value) {
		return fmt.Errorf("invalid join request id")
	}
	return nil
}

func validateJoinTokenID(value string) error {
	if !joinTokenIDPattern.MatchString(value) {
		return fmt.Errorf("invalid join token id")
	}
	return nil
}

func validateImage(value string) error {
	if strings.Contains(value, "://") || strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("invalid image")
	}
	if !imagePattern.MatchString(value) {
		return fmt.Errorf("image must be 1-255 characters and use a valid Docker image reference character set")
	}
	return nil
}

func validatePort(value int, label string) error {
	if value < 1 || value > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", label)
	}
	return nil
}

func validateProtocol(value string) error {
	switch value {
	case "", "tcp", "udp":
		return nil
	default:
		return fmt.Errorf("protocol must be tcp or udp")
	}
}

func validateEnvKey(value string) error {
	if !envKeyPattern.MatchString(value) {
		return fmt.Errorf("environment variable names must start with a letter or underscore and use letters, numbers, or underscores")
	}
	return nil
}

func validateEnvValue(value string) error {
	if len(value) > 4096 || strings.ContainsAny(value, "\x00") {
		return fmt.Errorf("environment variable values must be 4096 characters or less and cannot contain NUL bytes")
	}
	return nil
}

func validateVolumeHostPath(value string) error {
	cleaned := filepath.Clean(value)
	if value != cleaned || !strings.HasPrefix(cleaned, "/var/lib/deft/volumes/") {
		return fmt.Errorf("volume host paths must be absolute paths under /var/lib/deft/volumes")
	}
	if strings.ContainsAny(cleaned, "\x00\r\n") {
		return fmt.Errorf("volume host paths cannot contain control characters")
	}
	return nil
}

func validateVolumeContainerPath(value string) error {
	cleaned := filepath.Clean(value)
	if value != cleaned || !strings.HasPrefix(cleaned, "/") || cleaned == "/" {
		return fmt.Errorf("volume container paths must be absolute non-root paths")
	}
	if strings.ContainsAny(cleaned, "\x00\r\n") {
		return fmt.Errorf("volume container paths cannot contain control characters")
	}
	return nil
}

func validateRestartPolicy(value string) error {
	switch value {
	case "", "no", "always", "unless-stopped", "on-failure":
		return nil
	default:
		return fmt.Errorf("restart policy must be no, always, unless-stopped, or on-failure")
	}
}

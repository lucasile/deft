package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const maxJSONBodyBytes = 32 * 1024

var (
	containerNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)
	containerIDPattern   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:-]{0,127}$`)
	commandIDPattern     = regexp.MustCompile(`^[a-f0-9]{32}$`)
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

func validateImage(value string) error {
	if strings.Contains(value, "://") || strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("invalid image")
	}
	if !imagePattern.MatchString(value) {
		return fmt.Errorf("image must be 1-255 characters and use a valid Docker image reference character set")
	}
	return nil
}

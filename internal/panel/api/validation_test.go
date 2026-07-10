package api

import "testing"

func TestValidateContainerName(t *testing.T) {
	valid := []string{"minecraft", "server_1", "server.prod-1"}
	for _, value := range valid {
		if err := validateContainerName(value); err != nil {
			t.Fatalf("expected %q to be valid: %v", value, err)
		}
	}

	invalid := []string{"", "-bad", "bad/name", "bad name"}
	for _, value := range invalid {
		if err := validateContainerName(value); err == nil {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}

func TestValidateImage(t *testing.T) {
	valid := []string{"nginx:alpine", "ghcr.io/lucasile/deft-panel:latest", "minecraft/server@sha256:abcdef"}
	for _, value := range valid {
		if err := validateImage(value); err != nil {
			t.Fatalf("expected %q to be valid: %v", value, err)
		}
	}

	invalid := []string{"", "https://example.com/image", "bad image", "bad\nimage"}
	for _, value := range invalid {
		if err := validateImage(value); err == nil {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}

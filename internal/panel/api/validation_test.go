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

func TestValidateCreateContainerConfig(t *testing.T) {
	config, err := validateCreateContainerConfig(createContainerRequest{
		Ports: []portMappingRequest{
			{HostPort: 25565, ContainerPort: 25565, Protocol: "tcp"},
		},
		Env: []envVarRequest{
			{Key: "EULA", Value: "TRUE"},
		},
		Volumes: []volumeMountRequest{
			{HostPath: "/var/lib/deft/volumes/minecraft-1", ContainerPath: "/data", ReadOnly: true},
		},
		RestartPolicy: "unless-stopped",
	})
	if err != nil {
		t.Fatalf("validate config: %v", err)
	}
	if len(config.ports) != 1 || config.ports[0].GetProtocol() != "tcp" {
		t.Fatalf("unexpected ports: %+v", config.ports)
	}
	if len(config.env) != 1 || config.env[0].GetKey() != "EULA" {
		t.Fatalf("unexpected env: %+v", config.env)
	}
	if len(config.volumes) != 1 || !config.volumes[0].GetReadOnly() {
		t.Fatalf("unexpected volumes: %+v", config.volumes)
	}
	if config.restartPolicy != "unless-stopped" {
		t.Fatalf("restart policy = %q", config.restartPolicy)
	}
}

func TestValidateCreateContainerConfigRejectsUnsafeVolumeHostPath(t *testing.T) {
	_, err := validateCreateContainerConfig(createContainerRequest{
		Volumes: []volumeMountRequest{
			{HostPath: "/etc", ContainerPath: "/host"},
		},
	})
	if err == nil {
		t.Fatal("expected unsafe host path to be rejected")
	}
}

func TestValidateCreateContainerConfigRejectsBadRestartPolicy(t *testing.T) {
	_, err := validateCreateContainerConfig(createContainerRequest{RestartPolicy: "bad-policy"})
	if err == nil {
		t.Fatal("expected bad restart policy to be rejected")
	}
}

func TestValidateConsoleCommand(t *testing.T) {
	valid := []string{"say hello", "whitelist add player_1", "stop"}
	for _, value := range valid {
		if err := validateConsoleCommand(value); err != nil {
			t.Fatalf("expected %q to be valid: %v", value, err)
		}
	}

	invalid := []string{"", "say hello\nstop", "say hello\rstop", "bad\x00command"}
	for _, value := range invalid {
		if err := validateConsoleCommand(value); err == nil {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}

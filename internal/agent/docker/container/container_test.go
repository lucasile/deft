package container

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/lucasile/deft/internal/agent/docker"
)

func TestContainerLifecycle(t *testing.T) {
	ctx := context.Background()
	cli, err := docker.NewClient()
	if err != nil {
		t.Skipf("Skipping test because Docker daemon is not available: %v", err)
	}
	defer cli.Close()

	name := "deft-test-lifecycle"
	imageName := "nginx:alpine"

	id, err := Create(ctx, cli, name, imageName, &container.Config{
		Image: imageName,
	}, &container.HostConfig{})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	if err := Start(ctx, cli, id); err != nil {
		t.Errorf("Failed to start container: %v", err)
	}

	if err := Stop(ctx, cli, id); err != nil {
		t.Errorf("Failed to stop container: %v", err)
	}

	if err := Remove(ctx, cli, id); err != nil {
		t.Errorf("Failed to remove container: %v", err)
	}
}

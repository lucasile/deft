package docker

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestContainerLifecycle(t *testing.T) {
	ctx := context.Background()
	cli, err := NewClient()
	if err != nil {
		t.Skipf("Skipping test because Docker daemon is not available: %v", err)
	}
	defer cli.Close()

	name := "deft-test-lifecycle"
	imageName := "nginx:alpine"

	// 1. Create
	id, err := cli.CreateContainer(ctx, name, imageName, &container.Config{
		Image: imageName,
	}, &container.HostConfig{})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	// 2. Start
	if err := cli.StartContainer(ctx, id); err != nil {
		t.Errorf("Failed to start container: %v", err)
	}

	// 3. Stop
	if err := cli.StopContainer(ctx, id); err != nil {
		t.Errorf("Failed to stop container: %v", err)
	}

	// 4. Remove
	if err := cli.RemoveContainer(ctx, id); err != nil {
		t.Errorf("Failed to remove container: %v", err)
	}
}

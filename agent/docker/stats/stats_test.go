package stats

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/lucasile/deft/agent/docker"
	containerpkg "github.com/lucasile/deft/agent/docker/container"
)

func TestStreamStats(t *testing.T) {
	ctx := context.Background()
	cli, err := docker.NewClient()
	if err != nil {
		t.Skipf("Skipping test because Docker daemon is not available: %v", err)
	}
	defer cli.Close()

	name := "deft-test-stats"
	imageName := "alpine"

	id, err := containerpkg.Create(ctx, cli, name, imageName, &container.Config{
		Image: imageName,
		Cmd:   []string{"sh", "-c", "sleep 10"},
	}, &container.HostConfig{})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	defer func() {
		_ = containerpkg.Remove(ctx, cli, id)
	}()

	if err := containerpkg.Start(ctx, cli, id); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	reader, err := StreamStats(ctx, cli, id)
	if err != nil {
		t.Fatalf("Failed to stream stats: %v", err)
	}
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	var s container.StatsResponse
	
	errChan := make(chan error, 1)
	go func() {
		errChan <- decoder.Decode(&s)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Failed to decode stats: %v", err)
		}
		if s.ID != id {
			t.Errorf("Expected stats for ID %s, got %s", id, s.ID)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Timed out waiting for stats")
	}
}

package docker

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
)

func TestConsole(t *testing.T) {
	ctx := context.Background()
	cli, err := NewClient()
	if err != nil {
		t.Skipf("Skipping test because Docker daemon is not available: %v", err)
	}
	defer cli.Close()

	name := "deft-test-console"
	imageName := "alpine"

	id, err := cli.CreateContainer(ctx, name, imageName, &container.Config{
		Image:        imageName,
		Cmd:          []string{"sh", "-c", "echo 'hello world'; sleep 10"},
		OpenStdin:    true,
		StdinOnce:    false,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}, &container.HostConfig{})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	defer func() {
		_ = cli.RemoveContainer(ctx, id)
	}()

	if err := cli.StartContainer(ctx, id); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	reader, err := cli.StreamLogs(ctx, id)
	if err != nil {
		t.Fatalf("Failed to stream logs: %v", err)
	}
	defer reader.Close()

	found := false
	scanner := bufio.NewScanner(reader)
	
	timeout := time.After(5 * time.Second)
	lineChan := make(chan string)

	go func() {
		if scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	select {
	case line := <-lineChan:
		if strings.Contains(line, "hello world") {
			found = true
		}
	case <-timeout:
		t.Fatal("Timed out waiting for log output")
	}

	if !found {
		t.Error("Expected log message 'hello world' not found")
	}
}

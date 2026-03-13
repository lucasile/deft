package console

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/lucasile/deft/agent/docker"
	containerpkg "github.com/lucasile/deft/agent/docker/container"
)

func TestConsole(t *testing.T) {
	ctx := context.Background()
	cli, err := docker.NewClient()
	if err != nil {
		t.Skipf("Skipping test because Docker daemon is not available: %v", err)
	}
	defer cli.Close()

	name := "deft-test-console"
	imageName := "alpine"

	id, err := containerpkg.Create(ctx, cli, name, imageName, &container.Config{
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
		_ = containerpkg.Remove(ctx, cli, id)
	}()

	if err := containerpkg.Start(ctx, cli, id); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	reader, err := StreamLogs(ctx, cli, id)
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

func TestSendCommand(t *testing.T) {
	ctx := context.Background()
	cli, err := docker.NewClient()
	if err != nil {
		t.Skipf("Skipping test because Docker daemon is not available: %v", err)
	}
	defer cli.Close()

	name := "deft-test-send-command"
	imageName := "alpine"

	id, err := containerpkg.Create(ctx, cli, name, imageName, &container.Config{
		Image:       imageName,
		Cmd:         []string{"sh"},
		OpenStdin:   true,
		StdinOnce:   false,
		AttachStdin: true,
		Tty:         true,
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

	reader, err := StreamLogs(ctx, cli, id)
	if err != nil {
		t.Fatalf("Failed to stream logs: %v", err)
	}
	defer reader.Close()

	expected := "legit connection"
	if err := SendCommand(ctx, cli, id, "echo '"+expected+"'"); err != nil {
		t.Fatalf("Failed to send command: %v", err)
	}

	found := false
	scanner := bufio.NewScanner(reader)
	timeout := time.After(5 * time.Second)
	lineChan := make(chan string)

	go func() {
		for scanner.Scan() {
			lineChan <- scanner.Text()
		}
	}()

	for {
		select {
		case line := <-lineChan:
			if strings.Contains(line, expected) {
				found = true
				goto Done
			}
		case <-timeout:
			t.Fatal("Timed out waiting for command output")
		}
	}

Done:
	if !found {
		t.Errorf("Expected output '%s' not found", expected)
	}
}

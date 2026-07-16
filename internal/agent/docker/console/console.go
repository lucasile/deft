package console

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/lucasile/deft/internal/agent/docker"
)

func StreamLogs(ctx context.Context, c *docker.Client, id string) (io.ReadCloser, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	}

	return c.ContainerLogs(ctx, id, options)
}

func FollowLogs(ctx context.Context, c *docker.Client, id string, tailLines int, writeChunk func([]byte) error) error {
	if tailLines <= 0 {
		tailLines = 200
	}
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
		Tail:       strconv.Itoa(tailLines),
	}

	reader, err := c.ContainerLogs(ctx, id, options)
	if err != nil {
		return err
	}
	defer reader.Close()

	writer := chunkWriter{write: writeChunk}
	if _, err := stdcopy.StdCopy(writer, writer, reader); err != nil {
		return fmt.Errorf("failed to stream container logs: %w", err)
	}
	return nil
}

type chunkWriter struct {
	write func([]byte) error
}

func (w chunkWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	chunk := append([]byte(nil), p...)
	if err := w.write(chunk); err != nil {
		return 0, err
	}
	return len(p), nil
}

func FetchLogs(ctx context.Context, c *docker.Client, id string, tailLines int) (string, error) {
	if tailLines <= 0 {
		tailLines = 200
	}
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
		Timestamps: false,
		Tail:       strconv.Itoa(tailLines),
	}

	reader, err := c.ContainerLogs(ctx, id, options)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return "", fmt.Errorf("failed to read container logs: %w", err)
	}
	if stderr.Len() > 0 {
		stdout.Write(stderr.Bytes())
	}

	return stdout.String(), nil
}

func SendCommand(ctx context.Context, c *docker.Client, id string, command string) error {
	resp, err := c.ContainerAttach(ctx, id, container.AttachOptions{
		Stream: true,
		Stdin:  true,
	})
	if err != nil {
		return fmt.Errorf("failed to attach to container: %w", err)
	}
	defer resp.Close()

	_, err = fmt.Fprintln(resp.Conn, command)
	if err != nil {
		return fmt.Errorf("failed to write command to container: %w", err)
	}

	return nil
}

package console

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/lucasile/deft/agent/docker"
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

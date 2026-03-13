package container

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/lucasile/deft/internal/agent/docker"
	"github.com/rs/zerolog/log"
)

func Create(ctx context.Context, c *docker.Client, name, imgName string, config *container.Config, hostConfig *container.HostConfig) (string, error) {
	reader, err := c.ImagePull(ctx, imgName, image.PullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader)

	resp, err := c.ContainerCreate(ctx, config, hostConfig, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return resp.ID, nil
}

func Start(ctx context.Context, c *docker.Client, id string) error {
	if err := c.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	log.Debug().Str("id", id).Msg("Container started")
	return nil
}

func Stop(ctx context.Context, c *docker.Client, id string) error {
	timeout := 10
	if err := c.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	log.Debug().Str("id", id).Msg("Container stopped")
	return nil
}

func Remove(ctx context.Context, c *docker.Client, id string) error {
	if err := c.ContainerRemove(ctx, id, container.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}
	log.Debug().Str("id", id).Msg("Container removed")
	return nil
}

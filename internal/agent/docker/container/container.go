package container

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/lucasile/deft/internal/agent/docker"
	"github.com/rs/zerolog/log"
)

const (
	LabelManaged    = "deft.managed"
	LabelNodeID     = "deft.node_id"
	LabelName       = "deft.name"
	LabelResourceID = "deft.resource_id"
)

type Summary struct {
	ID         string
	Name       string
	Image      string
	Status     string
	ResourceID string
}

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

func ListManaged(ctx context.Context, c *docker.Client, nodeID string) ([]Summary, error) {
	containers, err := c.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", LabelManaged+"=true"),
			filters.Arg("label", LabelNodeID+"="+nodeID),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list managed containers: %w", err)
	}

	result := make([]Summary, 0, len(containers))
	for _, item := range containers {
		name := item.Labels[LabelName]
		if name == "" && len(item.Names) > 0 {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		result = append(result, Summary{
			ID:         item.ID,
			Name:       name,
			Image:      item.Image,
			Status:     panelStatus(item.State),
			ResourceID: item.Labels[LabelResourceID],
		})
	}

	return result, nil
}

func panelStatus(dockerState string) string {
	switch dockerState {
	case "running":
		return "running"
	case "exited":
		return "stopped"
	default:
		return dockerState
	}
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

func Restart(ctx context.Context, c *docker.Client, id string) error {
	timeout := 10
	if err := c.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to restart container: %w", err)
	}
	log.Debug().Str("id", id).Msg("Container restarted")
	return nil
}

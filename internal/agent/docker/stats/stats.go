package stats

import (
	"context"
	"io"

	"github.com/lucasile/deft/internal/agent/docker"
)

func StreamStats(ctx context.Context, c *docker.Client, id string) (io.ReadCloser, error) {
	resp, err := c.ContainerStats(ctx, id, true)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
	"github.com/rs/zerolog/log"
)

type Client struct {
	*client.Client
}

func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	c := &Client{Client: cli}

	if err := c.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to connect to docker: %w", err)
	}

	log.Info().Msg("Connected to Docker daemon successfully")
	return c, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Client.Ping(ctx)
	return err
}

func (c *Client) Close() error {
	return c.Client.Close()
}

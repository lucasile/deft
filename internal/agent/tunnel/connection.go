package tunnel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/lucasile/deft/proto"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Connection struct {
	client proto.AgentServiceClient
	stream proto.AgentService_ConnectClient
}

func NewConnection(ctx context.Context, addr string, caPath, certPath, keyPath string) (*Connection, error) {
	creds, err := loadTLSCredentials(caPath, certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to panel: %w", err)
	}

	client := proto.NewAgentServiceClient(conn)
	
	c := &Connection{
		client: client,
	}

	return c, nil
}

func (c *Connection) Connect(ctx context.Context, nodeID string) error {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		stream, err := c.client.Connect(ctx)
		if err != nil {
			log.Error().Err(err).Msgf("Connection failed, retrying in %v...", backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		c.stream = stream
		log.Info().Msg("Connected to panel via gRPC tunnel")
		
		// Reset backoff on successful connection
		backoff = 1 * time.Second
		
		// Send initial heartbeat to identify node
		if err := c.SendHeartbeat(nodeID); err != nil {
			log.Error().Err(err).Msg("Failed to send initial heartbeat")
			continue
		}

		return nil
	}
}

func (c *Connection) SendHeartbeat(nodeID string) error {
	msg := &proto.AgentMessage{
		NodeId: nodeID,
		Content: &proto.AgentMessage_Heartbeat{
			Heartbeat: &proto.Heartbeat{
				Timestamp: time.Now().Unix(),
			},
		},
	}
	return c.stream.Send(msg)
}

func (c *Connection) Receive() (*proto.PanelCommand, error) {
	return c.stream.Recv()
}

func loadTLSCredentials(caPath, certPath, keyPath string) (credentials.TransportCredentials, error) {
	pemServerCA, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemServerCA) {
		return nil, fmt.Errorf("failed to add server CA's certificate")
	}

	clientCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
	}

	return credentials.NewTLS(config), nil
}

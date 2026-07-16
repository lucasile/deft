package nodes

import (
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lucasile/deft/internal/panel/db"
	"github.com/lucasile/deft/internal/panel/events"
	"github.com/lucasile/deft/internal/proto"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

const (
	EventNodesChanged      = "nodes.changed"
	EventCommandUpdated    = "command.updated"
	EventContainersChanged = "containers.changed"
	EventLogChunk          = "logs.chunk"
)

type NodeConnection struct {
	stream proto.AgentService_ConnectServer
	nodeID string
}

type Node struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	LastSeen  int64  `json:"last_seen"`
	CreatedAt int64  `json:"created_at"`
	Connected bool   `json:"connected"`
}

type Command struct {
	ID          string `json:"id"`
	NodeID      string `json:"node_id"`
	Action      string `json:"action"`
	TargetID    string `json:"target_id,omitempty"`
	Status      string `json:"status"`
	Success     *bool  `json:"success,omitempty"`
	Message     string `json:"message,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	CompletedAt *int64 `json:"completed_at,omitempty"`
}

type Container struct {
	ID     string `json:"id"`
	NodeID string `json:"node_id"`
	Name   string `json:"name,omitempty"`
	Image  string `json:"image,omitempty"`
	Status string `json:"status,omitempty"`
}

type LogChunk struct {
	NodeID      string `json:"node_id"`
	StreamID    string `json:"stream_id"`
	ContainerID string `json:"container_id"`
	Data        []byte `json:"data,omitempty"`
	EOF         bool   `json:"eof,omitempty"`
	Error       string `json:"error,omitempty"`
}

type Manager struct {
	proto.UnimplementedAgentServiceServer
	mu                  sync.RWMutex
	nodes               map[string]*NodeConnection
	db                  *db.DB
	events              *events.Hub
	requireCertIdentity bool
}

func NewManager(database *db.DB, eventHub *events.Hub, requireCertIdentity bool) *Manager {
	return &Manager{
		nodes:               make(map[string]*NodeConnection),
		db:                  database,
		events:              eventHub,
		requireCertIdentity: requireCertIdentity,
	}
}

func (m *Manager) Connect(stream proto.AgentService_ConnectServer) error {
	msg, err := stream.Recv()
	if err != nil {
		return err
	}

	heartbeat := msg.GetHeartbeat()
	if heartbeat == nil {
		return fmt.Errorf("initial message must be a heartbeat")
	}

	nodeID := msg.NodeId
	if nodeID == "" {
		return fmt.Errorf("node id is required")
	}
	if m.requireCertIdentity {
		if err := m.verifyNodeCertificate(stream, nodeID); err != nil {
			log.Warn().Err(err).Str("node_id", nodeID).Msg("rejected agent certificate")
			return err
		}
	}

	m.mu.Lock()
	if _, exists := m.nodes[nodeID]; exists {
		m.mu.Unlock()
		log.Warn().Str("node_id", nodeID).Msg("rejected duplicate agent connection")
		return fmt.Errorf("node %s is already connected", nodeID)
	}

	conn := &NodeConnection{
		stream: stream,
		nodeID: nodeID,
	}

	m.nodes[nodeID] = conn
	m.mu.Unlock()
	log.Info().Str("node_id", nodeID).Msg("Agent connected")
	m.publish(EventNodesChanged, map[string]string{"node_id": nodeID})

	defer func() {
		m.mu.Lock()
		delete(m.nodes, nodeID)
		m.mu.Unlock()
		log.Info().Str("node_id", nodeID).Msg("Agent disconnected")
		m.publish(EventNodesChanged, map[string]string{"node_id": nodeID})
	}()

	_, err = m.db.Exec(
		`INSERT INTO nodes (id, last_seen)
		 VALUES (?, ?)
		 ON CONFLICT(id) DO UPDATE SET last_seen = excluded.last_seen`,
		nodeID,
		time.Now().Unix(),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to update node in db")
	}

	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}

		if res := msg.GetResult(); res != nil {
			log.Info().Str("node_id", nodeID).Str("command_id", res.CommandId).Bool("success", res.Success).Msg("Received command result")
			if err := m.CompleteCommand(res.CommandId, res.Success, res.Message); err != nil {
				log.Error().Err(err).Str("command_id", res.CommandId).Msg("failed to update command result")
			}
			m.publish(EventCommandUpdated, map[string]string{"command_id": res.CommandId})
		}

		if inventory := msg.GetInventory(); inventory != nil {
			if err := m.SyncContainers(nodeID, inventory.Containers); err != nil {
				log.Error().Err(err).Str("node_id", nodeID).Msg("failed to sync container inventory")
			}
		}

		if chunk := msg.GetLogChunk(); chunk != nil {
			m.publish(EventLogChunk, LogChunk{
				NodeID:      nodeID,
				StreamID:    chunk.GetStreamId(),
				ContainerID: chunk.GetContainerId(),
				Data:        chunk.GetData(),
				EOF:         chunk.GetEof(),
				Error:       chunk.GetError(),
			})
		}

		if _, err := m.db.Exec("UPDATE nodes SET last_seen = ? WHERE id = ?", time.Now().Unix(), nodeID); err != nil {
			log.Error().Err(err).Str("node_id", nodeID).Msg("failed to update node heartbeat")
		}
	}
}

func (m *Manager) SendCommand(nodeID string, cmd *proto.PanelCommand) error {
	m.mu.RLock()
	node, ok := m.nodes[nodeID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("node %s not connected", nodeID)
	}

	return node.stream.Send(cmd)
}

func (m *Manager) verifyNodeCertificate(stream proto.AgentService_ConnectServer, nodeID string) error {
	cert, err := peerCertificate(stream)
	if err != nil {
		return err
	}

	certNodeID := nodeIDFromCertificate(cert)
	if certNodeID == "" {
		return fmt.Errorf("client certificate is missing deft node identity")
	}
	if certNodeID != nodeID {
		return fmt.Errorf("client certificate node identity does not match requested node id")
	}

	fingerprintBytes := sha256.Sum256(cert.Raw)
	fingerprint := hex.EncodeToString(fingerprintBytes[:])

	var storedFingerprint sql.NullString
	err = m.db.QueryRow(
		"SELECT cert_fingerprint FROM nodes WHERE id = ?",
		nodeID,
	).Scan(&storedFingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("node %s is not joined", nodeID)
	}
	if err != nil {
		return fmt.Errorf("failed to load node certificate fingerprint: %w", err)
	}
	if !storedFingerprint.Valid || storedFingerprint.String != fingerprint {
		return fmt.Errorf("client certificate fingerprint does not match joined node")
	}

	return nil
}

func peerCertificate(stream proto.AgentService_ConnectServer) (*x509.Certificate, error) {
	p, ok := peer.FromContext(stream.Context())
	if !ok {
		return nil, fmt.Errorf("missing gRPC peer")
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, fmt.Errorf("missing TLS peer info")
	}
	if len(tlsInfo.State.PeerCertificates) == 0 {
		return nil, fmt.Errorf("missing client certificate")
	}
	return tlsInfo.State.PeerCertificates[0], nil
}

func nodeIDFromCertificate(cert *x509.Certificate) string {
	for _, uri := range cert.URIs {
		if uri.Scheme == "deft" && uri.Opaque != "" {
			if value, ok := strings.CutPrefix(uri.Opaque, "node:"); ok {
				return value
			}
		}
	}
	return ""
}

func (m *Manager) ListNodes() ([]Node, error) {
	rows, err := m.db.Query("SELECT id, name, last_seen, COALESCE(created_at, last_seen, 0) FROM nodes ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	defer rows.Close()

	m.mu.RLock()
	connected := make(map[string]bool, len(m.nodes))
	for nodeID := range m.nodes {
		connected[nodeID] = true
	}
	m.mu.RUnlock()

	var result []Node
	for rows.Next() {
		var node Node
		var name sql.NullString
		if err := rows.Scan(&node.ID, &name, &node.LastSeen, &node.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		if name.Valid {
			node.Name = name.String
		}
		node.Connected = connected[node.ID]
		result = append(result, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read nodes: %w", err)
	}

	return result, nil
}

func (m *Manager) RemoveNode(nodeID string) error {
	m.mu.RLock()
	_, connected := m.nodes[nodeID]
	m.mu.RUnlock()
	if connected {
		return fmt.Errorf("node is connected")
	}

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start node removal: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE join_tokens SET used_by_node_id = NULL WHERE used_by_node_id = ?", nodeID); err != nil {
		return fmt.Errorf("failed to detach join tokens: %w", err)
	}
	if _, err := tx.Exec("UPDATE join_requests SET approved_node_id = NULL WHERE approved_node_id = ?", nodeID); err != nil {
		return fmt.Errorf("failed to detach join requests: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM containers WHERE node_id = ?", nodeID); err != nil {
		return fmt.Errorf("failed to delete node containers: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM commands WHERE node_id = ?", nodeID); err != nil {
		return fmt.Errorf("failed to delete node commands: %w", err)
	}
	result, err := tx.Exec("DELETE FROM nodes WHERE id = ?", nodeID)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to confirm node removal: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("node not found")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit node removal: %w", err)
	}

	m.publish(EventNodesChanged, map[string]string{"node_id": nodeID})
	m.publish(EventContainersChanged, map[string]string{"node_id": nodeID})
	return nil
}

func (m *Manager) UpsertContainer(nodeID, containerID, name, image, status string) error {
	_, err := m.db.Exec(
		`INSERT INTO containers (id, node_id, name, image, status)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   node_id = excluded.node_id,
		   name = excluded.name,
		   image = excluded.image,
		   status = excluded.status`,
		containerID,
		nodeID,
		name,
		image,
		status,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert container: %w", err)
	}

	m.publish(EventContainersChanged, map[string]string{"node_id": nodeID, "container_id": containerID})
	return nil
}

func (m *Manager) publish(name string, data any) {
	if m.events != nil {
		m.events.Publish(name, data)
	}
}

func (m *Manager) UpdateContainerStatus(nodeID, containerID, status string) error {
	result, err := m.db.Exec(
		`UPDATE containers
		 SET status = ?
		 WHERE node_id = ? AND id = ?`,
		status,
		nodeID,
		containerID,
	)
	if err != nil {
		return fmt.Errorf("failed to update container status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to confirm container status update: %w", err)
	}
	if rowsAffected == 0 {
		return nil
	}

	m.publish(EventContainersChanged, map[string]string{"node_id": nodeID, "container_id": containerID})
	return nil
}

func (m *Manager) ListContainers(nodeID string) ([]Container, error) {
	rows, err := m.db.Query(
		`SELECT id, node_id, name, image, status
		 FROM containers
		 WHERE node_id = ?
		 ORDER BY name, id`,
		nodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	defer rows.Close()

	containers := []Container{}
	for rows.Next() {
		var container Container
		var name sql.NullString
		var image sql.NullString
		var status sql.NullString
		if err := rows.Scan(&container.ID, &container.NodeID, &name, &image, &status); err != nil {
			return nil, fmt.Errorf("failed to scan container: %w", err)
		}
		if name.Valid {
			container.Name = name.String
		}
		if image.Valid {
			container.Image = image.String
		}
		if status.Valid {
			container.Status = status.String
		}
		containers = append(containers, container)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read containers: %w", err)
	}

	return containers, nil
}

func (m *Manager) SyncContainers(nodeID string, inventory []*proto.ContainerSummary) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start container sync: %w", err)
	}
	defer tx.Rollback()

	ids := make([]string, 0, len(inventory))
	for _, item := range inventory {
		if item.GetId() == "" {
			continue
		}
		ids = append(ids, item.GetId())

		if item.GetName() != "" {
			if _, err := tx.Exec(
				`DELETE FROM containers
				 WHERE node_id = ? AND id = ?`,
				nodeID,
				item.GetName(),
			); err != nil {
				return fmt.Errorf("failed to remove stale container placeholder: %w", err)
			}
		}

		if _, err := tx.Exec(
			`INSERT INTO containers (id, node_id, name, image, status)
			 VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET
			   node_id = excluded.node_id,
			   name = excluded.name,
			   image = excluded.image,
			   status = excluded.status`,
			item.GetId(),
			nodeID,
			item.GetName(),
			item.GetImage(),
			item.GetStatus(),
		); err != nil {
			return fmt.Errorf("failed to sync container: %w", err)
		}
	}

	if len(ids) == 0 {
		if _, err := tx.Exec(
			`DELETE FROM containers
			 WHERE node_id = ?`,
			nodeID,
		); err != nil {
			return fmt.Errorf("failed to delete missing containers: %w", err)
		}
	} else {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
		args := make([]any, 0, len(ids)+1)
		args = append(args, nodeID)
		for _, id := range ids {
			args = append(args, id)
		}
		if _, err := tx.Exec(
			`DELETE FROM containers
			 WHERE node_id = ? AND id NOT IN (`+placeholders+`)`,
			args...,
		); err != nil {
			return fmt.Errorf("failed to delete missing containers: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit container sync: %w", err)
	}

	m.publish(EventContainersChanged, map[string]string{"node_id": nodeID})
	return nil
}

func (m *Manager) CreateCommand(commandID, nodeID, action, targetID string) error {
	_, err := m.db.Exec(
		`INSERT INTO commands (id, node_id, action, target_id, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		commandID,
		nodeID,
		action,
		targetID,
		"pending",
		time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("failed to create command record: %w", err)
	}

	return nil
}

func (m *Manager) CompleteCommand(commandID string, success bool, message string) error {
	var nodeID string
	var action string
	var targetID sql.NullString
	err := m.db.QueryRow(
		"SELECT node_id, action, target_id FROM commands WHERE id = ?",
		commandID,
	).Scan(&nodeID, &action, &targetID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("command %s not found", commandID)
	}
	if err != nil {
		return fmt.Errorf("failed to load command: %w", err)
	}

	status := "failed"
	if success {
		status = "succeeded"
	}

	_, err = m.db.Exec(
		`UPDATE commands
		 SET status = ?, success = ?, message = ?, completed_at = ?
		 WHERE id = ?`,
		status,
		boolToInt(success),
		message,
		time.Now().Unix(),
		commandID,
	)
	if err != nil {
		return fmt.Errorf("failed to complete command: %w", err)
	}

	if success && targetID.Valid {
		if action == "container.remove" {
			if err := m.DeleteContainer(nodeID, targetID.String); err != nil {
				return err
			}
		} else if containerStatus := finalContainerStatus(action); containerStatus != "" {
			if err := m.UpdateContainerStatus(nodeID, targetID.String, containerStatus); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manager) DeleteContainer(nodeID, containerID string) error {
	_, err := m.db.Exec(
		`DELETE FROM containers
		 WHERE node_id = ? AND id = ?`,
		nodeID,
		containerID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete container: %w", err)
	}

	m.publish(EventContainersChanged, map[string]string{"node_id": nodeID, "container_id": containerID})
	return nil
}

func (m *Manager) GetCommand(commandID string) (*Command, error) {
	var command Command
	var targetID sql.NullString
	var success sql.NullBool
	var message sql.NullString
	var completedAt sql.NullInt64

	err := m.db.QueryRow(
		`SELECT id, node_id, action, target_id, status, success, message, created_at, completed_at
		 FROM commands
		 WHERE id = ?`,
		commandID,
	).Scan(
		&command.ID,
		&command.NodeID,
		&command.Action,
		&targetID,
		&command.Status,
		&success,
		&message,
		&command.CreatedAt,
		&completedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("command %s not found", commandID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get command: %w", err)
	}

	if targetID.Valid {
		command.TargetID = targetID.String
	}
	if success.Valid {
		value := success.Bool
		command.Success = &value
	}
	if message.Valid {
		command.Message = message.String
	}
	if completedAt.Valid {
		value := completedAt.Int64
		command.CompletedAt = &value
	}

	return &command, nil
}

func (m *Manager) ListCommands(limit int) ([]Command, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	rows, err := m.db.Query(
		`SELECT id, node_id, action, target_id, status, success, message, created_at, completed_at
		 FROM commands
		 ORDER BY created_at DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list commands: %w", err)
	}
	defer rows.Close()

	commands := []Command{}
	for rows.Next() {
		command, err := scanCommand(rows)
		if err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read commands: %w", err)
	}

	return commands, nil
}

type commandScanner interface {
	Scan(dest ...any) error
}

func scanCommand(scanner commandScanner) (Command, error) {
	var command Command
	var targetID sql.NullString
	var success sql.NullBool
	var message sql.NullString
	var completedAt sql.NullInt64

	if err := scanner.Scan(
		&command.ID,
		&command.NodeID,
		&command.Action,
		&targetID,
		&command.Status,
		&success,
		&message,
		&command.CreatedAt,
		&completedAt,
	); err != nil {
		return Command{}, fmt.Errorf("failed to scan command: %w", err)
	}

	if targetID.Valid {
		command.TargetID = targetID.String
	}
	if success.Valid {
		value := success.Bool
		command.Success = &value
	}
	if message.Valid {
		command.Message = message.String
	}
	if completedAt.Valid {
		value := completedAt.Int64
		command.CompletedAt = &value
	}

	return command, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func finalContainerStatus(action string) string {
	switch action {
	case "container.create":
		return "created"
	case "container.start":
		return "running"
	case "container.stop":
		return "stopped"
	default:
		return ""
	}
}

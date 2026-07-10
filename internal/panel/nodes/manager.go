package nodes

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lucasile/deft/internal/panel/db"
	"github.com/lucasile/deft/internal/proto"
	"github.com/rs/zerolog/log"
)

type NodeConnection struct {
	stream proto.AgentService_ConnectServer
	nodeID string
}

type Node struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	LastSeen  int64  `json:"last_seen"`
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

type Manager struct {
	proto.UnimplementedAgentServiceServer
	mu    sync.RWMutex
	nodes map[string]*NodeConnection
	db    *db.DB
}

func NewManager(database *db.DB) *Manager {
	return &Manager{
		nodes: make(map[string]*NodeConnection),
		db:    database,
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

	defer func() {
		m.mu.Lock()
		delete(m.nodes, nodeID)
		m.mu.Unlock()
		log.Info().Str("node_id", nodeID).Msg("Agent disconnected")
	}()

	_, err = m.db.Exec("INSERT OR REPLACE INTO nodes (id, last_seen) VALUES (?, ?)", nodeID, time.Now().Unix())
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

func (m *Manager) ListNodes() ([]Node, error) {
	rows, err := m.db.Query("SELECT id, name, last_seen FROM nodes ORDER BY id")
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
		if err := rows.Scan(&node.ID, &name, &node.LastSeen); err != nil {
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

	return nil
}

func (m *Manager) UpdateContainerStatus(nodeID, containerID, status string) error {
	_, err := m.db.Exec(
		`INSERT INTO containers (id, node_id, name, status)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   status = excluded.status`,
		containerID,
		nodeID,
		containerID,
		status,
	)
	if err != nil {
		return fmt.Errorf("failed to update container status: %w", err)
	}

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
		if containerStatus := finalContainerStatus(action); containerStatus != "" {
			if err := m.UpdateContainerStatus(nodeID, targetID.String, containerStatus); err != nil {
				return err
			}
		}
	}

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
	case "container.remove":
		return "removed"
	default:
		return ""
	}
}

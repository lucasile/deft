package nodes

import (
	"fmt"
	"sync"
	"time"

	"github.com/lucasile/deft/internal/panel/db"
	"github.com/lucasile/deft/proto"
	"github.com/rs/zerolog/log"
)

type NodeConnection struct {
	stream proto.AgentService_ConnectServer
	nodeID string
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
	log.Info().Str("node_id", nodeID).Msg("Agent connected")

	conn := &NodeConnection{
		stream: stream,
		nodeID: nodeID,
	}

	m.mu.Lock()
	m.nodes[nodeID] = conn
	m.mu.Unlock()

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
		}

		_, _ = m.db.Exec("UPDATE nodes SET last_seen = ? WHERE id = ?", time.Now().Unix(), nodeID)
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

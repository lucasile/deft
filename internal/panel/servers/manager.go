package servers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lucasile/deft/internal/panel/db"
)

type Server struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	NodeID            string `json:"node_id"`
	ContainerID       string `json:"container_id,omitempty"`
	Image             string `json:"image"`
	Status            string `json:"status"`
	DesiredConfigJSON string `json:"desired_config_json"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
}

type CreateRequest struct {
	ID            string
	Name          string
	NodeID        string
	Image         string
	Status        string
	DesiredConfig any
}

type Manager struct {
	db *db.DB
}

func NewManager(database *db.DB) *Manager {
	return &Manager{db: database}
}

func (m *Manager) Create(req CreateRequest) error {
	status := req.Status
	if status == "" {
		status = "create_requested"
	}
	desiredConfig, err := json.Marshal(req.DesiredConfig)
	if err != nil {
		return fmt.Errorf("failed to encode desired server config: %w", err)
	}
	now := time.Now().Unix()

	_, err = m.db.Exec(
		`INSERT INTO servers (id, name, node_id, image, status, desired_config_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ID,
		req.Name,
		req.NodeID,
		req.Image,
		status,
		string(desiredConfig),
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	return nil
}

func (m *Manager) List() ([]Server, error) {
	rows, err := m.db.Query(
		`SELECT id, name, node_id, container_id, image, status, desired_config_json, created_at, updated_at
		 FROM servers
		 ORDER BY created_at DESC, id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}
	defer rows.Close()

	result := []Server{}
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, server)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read servers: %w", err)
	}

	return result, nil
}

func (m *Manager) Get(id string) (*Server, error) {
	server, err := scanServer(m.db.QueryRow(
		`SELECT id, name, node_id, container_id, image, status, desired_config_json, created_at, updated_at
		 FROM servers
		 WHERE id = ?`,
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("server %s not found", id)
	}
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func (m *Manager) UpdateStatus(id, status string) error {
	result, err := m.db.Exec(
		`UPDATE servers
		 SET status = ?, updated_at = ?
		 WHERE id = ?`,
		status,
		time.Now().Unix(),
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to confirm server status update: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("server %s not found", id)
	}
	return nil
}

type serverScanner interface {
	Scan(dest ...any) error
}

func scanServer(scanner serverScanner) (Server, error) {
	var server Server
	var containerID sql.NullString
	if err := scanner.Scan(
		&server.ID,
		&server.Name,
		&server.NodeID,
		&containerID,
		&server.Image,
		&server.Status,
		&server.DesiredConfigJSON,
		&server.CreatedAt,
		&server.UpdatedAt,
	); err != nil {
		return Server{}, fmt.Errorf("failed to scan server: %w", err)
	}
	if containerID.Valid {
		server.ContainerID = containerID.String
	}
	return server, nil
}

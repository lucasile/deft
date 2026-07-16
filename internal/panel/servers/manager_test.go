package servers

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lucasile/deft/internal/panel/db"
)

func TestManagerCreateListGet(t *testing.T) {
	tempDir := t.TempDir()
	database, err := db.Init(filepath.Join(tempDir, "panel.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(
		"INSERT INTO nodes (id, name, last_seen) VALUES (?, ?, ?)",
		"node-a",
		"Node A",
		time.Now().Unix(),
	); err != nil {
		t.Fatalf("insert node: %v", err)
	}

	manager := NewManager(database)
	if err := manager.Create(CreateRequest{
		ID:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Name:   "minecraft-1",
		NodeID: "node-a",
		Image:  "itzg/minecraft-server:latest",
		DesiredConfig: map[string]any{
			"restart_policy": "unless-stopped",
		},
	}); err != nil {
		t.Fatalf("create server: %v", err)
	}

	servers, err := manager.List()
	if err != nil {
		t.Fatalf("list servers: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("server count = %d, want 1", len(servers))
	}
	if servers[0].Status != "create_requested" {
		t.Fatalf("status = %q, want create_requested", servers[0].Status)
	}

	server, err := manager.Get("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if server.Name != "minecraft-1" || server.Image != "itzg/minecraft-server:latest" {
		t.Fatalf("unexpected server: %+v", server)
	}
}

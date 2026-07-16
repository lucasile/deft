package nodes

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lucasile/deft/internal/panel/db"
)

func TestListContainersForNode(t *testing.T) {
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

	manager := NewManager(database, nil, false)
	if err := manager.UpsertContainer("node-a", "hello-nginx", "hello-nginx", "nginx:alpine", "created"); err != nil {
		t.Fatalf("upsert container: %v", err)
	}
	if err := manager.UpdateContainerStatus("node-a", "hello-nginx", "started"); err != nil {
		t.Fatalf("update status: %v", err)
	}

	containers, err := manager.ListContainers("node-a")
	if err != nil {
		t.Fatalf("list containers: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("container count = %d, want 1", len(containers))
	}
	if containers[0].ID != "hello-nginx" || containers[0].Status != "started" {
		t.Fatalf("unexpected container: %+v", containers[0])
	}

	containers, err = manager.ListContainers("node-b")
	if err != nil {
		t.Fatalf("list missing node containers: %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("missing node container count = %d, want 0", len(containers))
	}
}

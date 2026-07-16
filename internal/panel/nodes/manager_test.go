package nodes

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/lucasile/deft/internal/panel/db"
	"github.com/lucasile/deft/internal/proto"
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

func TestSyncContainersAllowsDuplicateDisplayNames(t *testing.T) {
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
	err = manager.SyncContainers("node-a", []*proto.ContainerSummary{
		{Id: "docker-id-a", Name: "hello-nginx", Image: "nginx:alpine", Status: "created"},
		{Id: "docker-id-b", Name: "hello-nginx", Image: "nginx:alpine", Status: "created"},
	})
	if err != nil {
		t.Fatalf("sync containers: %v", err)
	}

	containers, err := manager.ListContainers("node-a")
	if err != nil {
		t.Fatalf("list containers: %v", err)
	}
	if len(containers) != 2 {
		t.Fatalf("container count = %d, want 2: %+v", len(containers), containers)
	}
}

func TestSyncContainersReplacesPlaceholdersAndDeletesMissing(t *testing.T) {
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
	if err := manager.UpsertContainer("node-a", "hello-nginx", "hello-nginx", "nginx:alpine", "create_requested"); err != nil {
		t.Fatalf("upsert placeholder: %v", err)
	}
	if err := manager.UpsertContainer("node-a", "old-container", "old-container", "busybox", "running"); err != nil {
		t.Fatalf("upsert old container: %v", err)
	}

	err = manager.SyncContainers("node-a", []*proto.ContainerSummary{
		{
			Id:     "docker-container-id",
			Name:   "hello-nginx",
			Image:  "nginx:alpine",
			Status: "created",
		},
	})
	if err != nil {
		t.Fatalf("sync containers: %v", err)
	}

	containers, err := manager.ListContainers("node-a")
	if err != nil {
		t.Fatalf("list containers: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("container count = %d, want 1: %+v", len(containers), containers)
	}

	byID := map[string]Container{}
	for _, container := range containers {
		byID[container.ID] = container
	}
	if _, ok := byID["hello-nginx"]; ok {
		t.Fatalf("placeholder container was not replaced: %+v", containers)
	}
	if byID["docker-container-id"].Status != "created" {
		t.Fatalf("synced container status = %q", byID["docker-container-id"].Status)
	}
	if _, ok := byID["old-container"]; ok {
		t.Fatalf("missing container was not deleted: %+v", containers)
	}
}

func TestSyncContainersLinksServerByResourceID(t *testing.T) {
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
	if _, err := database.Exec(
		`INSERT INTO servers (id, name, node_id, image, status, desired_config_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"resource-a",
		"minecraft-1",
		"node-a",
		"itzg/minecraft-server:latest",
		"create_requested",
		"{}",
		time.Now().Unix(),
		time.Now().Unix(),
	); err != nil {
		t.Fatalf("insert server: %v", err)
	}

	manager := NewManager(database, nil, false)
	if err := manager.SyncContainers("node-a", []*proto.ContainerSummary{
		{
			Id:         "docker-container-id",
			Name:       "minecraft-1",
			Image:      "itzg/minecraft-server:latest",
			Status:     "running",
			ResourceId: "resource-a",
		},
	}); err != nil {
		t.Fatalf("sync containers: %v", err)
	}

	var containerID string
	var status string
	if err := database.QueryRow("SELECT container_id, status FROM servers WHERE id = ?", "resource-a").Scan(&containerID, &status); err != nil {
		t.Fatalf("load server: %v", err)
	}
	if containerID != "docker-container-id" || status != "running" {
		t.Fatalf("server link = (%q, %q), want docker-container-id/running", containerID, status)
	}
}

func TestSyncContainersDoesNotMarkRemovingServerMissing(t *testing.T) {
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
	if _, err := database.Exec(
		`INSERT INTO containers (id, node_id, name, image, status)
		 VALUES (?, ?, ?, ?, ?)`,
		"docker-container-id",
		"node-a",
		"minecraft-1",
		"itzg/minecraft-server:latest",
		"running",
	); err != nil {
		t.Fatalf("insert container: %v", err)
	}
	if _, err := database.Exec(
		`INSERT INTO servers (id, name, node_id, container_id, image, status, desired_config_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"resource-a",
		"minecraft-1",
		"node-a",
		"docker-container-id",
		"itzg/minecraft-server:latest",
		"remove_requested",
		"{}",
		time.Now().Unix(),
		time.Now().Unix(),
	); err != nil {
		t.Fatalf("insert server: %v", err)
	}

	manager := NewManager(database, nil, false)
	if err := manager.SyncContainers("node-a", nil); err != nil {
		t.Fatalf("sync containers: %v", err)
	}

	var status string
	var containerID sql.NullString
	if err := database.QueryRow("SELECT container_id, status FROM servers WHERE id = ?", "resource-a").Scan(&containerID, &status); err != nil {
		t.Fatalf("load server: %v", err)
	}
	if status != "remove_requested" {
		t.Fatalf("status = %q, want remove_requested", status)
	}
	if containerID.Valid {
		t.Fatalf("container_id = %q, want null after stale container deletion", containerID.String)
	}
}

func TestDeleteContainerRemovesActiveContainer(t *testing.T) {
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
	if err := manager.UpsertContainer("node-a", "container-a", "container-a", "nginx:alpine", "created"); err != nil {
		t.Fatalf("upsert container: %v", err)
	}
	if err := manager.DeleteContainer("node-a", "container-a"); err != nil {
		t.Fatalf("delete container: %v", err)
	}

	containers, err := manager.ListContainers("node-a")
	if err != nil {
		t.Fatalf("list containers: %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("container count = %d, want 0", len(containers))
	}
}

func TestRemoveNodeDeletesOfflineNodeRecords(t *testing.T) {
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
	if err := manager.UpsertContainer("node-a", "container-a", "container-a", "nginx:alpine", "created"); err != nil {
		t.Fatalf("upsert container: %v", err)
	}
	if err := manager.CreateCommand("command-a", "node-a", "container.create", "container-a"); err != nil {
		t.Fatalf("create command: %v", err)
	}

	if err := manager.RemoveNode("node-a"); err != nil {
		t.Fatalf("remove node: %v", err)
	}

	nodes, err := manager.ListNodes()
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("node count = %d, want 0", len(nodes))
	}

	containers, err := manager.ListContainers("node-a")
	if err != nil {
		t.Fatalf("list containers: %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("container count = %d, want 0", len(containers))
	}
}

func TestRemoveNodeRejectsConnectedNode(t *testing.T) {
	tempDir := t.TempDir()
	database, err := db.Init(filepath.Join(tempDir, "panel.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer database.Close()

	manager := NewManager(database, nil, false)
	manager.nodes["node-a"] = &NodeConnection{nodeID: "node-a"}

	if err := manager.RemoveNode("node-a"); err == nil {
		t.Fatalf("expected connected node removal to fail")
	}
}

package recipes

import "testing"

func TestRenderMinecraftJava(t *testing.T) {
	rendered, err := Render("minecraft-java", "server123", map[string]any{
		"server_name": "survival",
		"memory_mb":   float64(4096),
		"version":     "1.21.1",
		"motd":        "hello",
		"difficulty":  "hard",
		"port":        float64(25566),
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	if rendered.Name != "survival" {
		t.Fatalf("Name = %q, want survival", rendered.Name)
	}
	if rendered.Image != "itzg/minecraft-server:latest" {
		t.Fatalf("Image = %q", rendered.Image)
	}
	if len(rendered.Ports) != 1 || rendered.Ports[0].HostPort != 25566 || rendered.Ports[0].ContainerPort != 25565 {
		t.Fatalf("Ports = %#v", rendered.Ports)
	}
	if len(rendered.Volumes) != 1 || rendered.Volumes[0].HostPath != "/var/lib/deft/volumes/server123" {
		t.Fatalf("Volumes = %#v", rendered.Volumes)
	}
}

func TestRenderRejectsUnknownInput(t *testing.T) {
	_, err := Render("minecraft-java", "server123", map[string]any{"bad": "value"})
	if err == nil {
		t.Fatal("Render returned nil error for unknown input")
	}
}

func TestRenderValidatesNumberBounds(t *testing.T) {
	_, err := Render("minecraft-java", "server123", map[string]any{"memory_mb": 128})
	if err == nil {
		t.Fatal("Render returned nil error for memory below minimum")
	}
}

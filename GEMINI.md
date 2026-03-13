# Deft — AI Assistant Context

## What Is Deft

Deft is an open source server management panel for game servers and general Docker applications. Two components: a **central web panel** and a **lightweight Go agent** that runs on each server node.

Core differentiator: the agent connects **outbound** to the panel — no inbound ports needed on game server nodes. Think Tailscale's model applied to server management.

North star: install panel, add a node, have a running Minecraft server in under 10 minutes.

---

## Developer Context

- Solo developer, Go beginner (experienced in Java, TypeScript, Python, C#)
- Always explain Go-specific idioms and patterns when using them
- Prefer simple readable code over clever abstractions
- Prefer standard library over third party where reasonable
- Never ignore errors with `_` — always handle explicitly
- Wrap errors with `fmt.Errorf("context: %w", err)`
- Use Zerolog for all logging, never fmt.Println in production code

---

## Architecture

```
[Browser]
    ↕ HTTPS + WebSocket
[Panel — Go binary with embedded SvelteKit SPA]
    ↕ gRPC over mutual TLS (agent connects OUTBOUND)
[Agent — Go binary on node]
    ↕ Unix socket (/var/run/docker.sock)
[Docker Engine]
    ↕
[Game Server Containers]
```

### Agent

- Single static Go binary, zero runtime dependencies, no CGO
- Runs directly on host OS — NOT inside Docker
- Managed by systemd as a background service
- Connects outbound to panel via persistent gRPC stream
- Only communicates with Docker socket and panel — nothing else
- Hardcoded command allowlist — cannot execute arbitrary host OS commands
- All file operations strictly scoped to `/var/lib/deft/volumes/`

### Panel

- Go binary serving REST API + gRPC server for agent connections
- SvelteKit frontend built to static SPA, embedded in Go binary via `embed.FS`
- Single binary deployment — no Node.js on server, no separate web server
- SQLite default (zero config), Postgres optional

### Communication

- Protocol: gRPC bidirectional streaming
- Security: mutual TLS — both sides verify certificates
- Direction: agent calls out to panel, never the reverse
- Reconnection: exponential backoff 1s → 60s cap
- Panel down: game servers keep running, agent reconnects automatically

---

## Tech Stack

| Layer | Technology |
|---|---|
| Agent + Panel backend | Go 1.22+ |
| Panel frontend | SvelteKit (SPA mode) + Tailwind CSS |
| Agent-Panel protocol | gRPC + Protocol Buffers |
| CLI | Cobra |
| Config | Viper |
| Database | SQLite via `modernc.org/sqlite` (pure Go, no CGO) |
| Logging | Zerolog |
| Auth | JWT |
| HTTP router | chi |
| Container management | `github.com/docker/docker/client` |
| Init system | systemd |

### Critical Package Notes

- Use `modernc.org/sqlite` NOT `mattn/go-sqlite3` — pure Go, no CGO, required for single static binary
- SvelteKit runs in SPA/static mode only — no SSR, no Node.js server
- Docker client import path is `github.com/docker/docker/client` — the Go module path does NOT change even though the GitHub source repo lives at github.com/moby/moby
- Never shell out to the docker CLI — always use the Docker API via the client package

### Docker Client Usage

```go
// Install
// go get github.com/docker/docker@latest

// Import
import "github.com/docker/docker/client"

// Create client (reads DOCKER_HOST env or defaults to /var/run/docker.sock)
cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
if err != nil {
    return fmt.Errorf("failed to create docker client: %w", err)
}
defer cli.Close()

// List containers
containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})

// Start container
err = cli.ContainerStart(ctx, containerID, container.StartOptions{})

// Stop container
timeout := 10
err = cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})

// Stream logs
logs, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{
    ShowStdout: true,
    ShowStderr: true,
    Follow:     true,
    Timestamps: false,
})

// Docs: https://pkg.go.dev/github.com/docker/docker/client
// Source: https://github.com/moby/moby (module path stays github.com/docker/docker)
```

---

## Repository Structure

```
deft/
├── agent/                  # Go agent binary
│   ├── main.go
│   ├── go.mod
│   ├── cmd/                # CLI commands (install, uninstall, serve, status)
│   ├── docker/             # Docker API wrapper (client, container, console, stats)
│   ├── tunnel/             # gRPC connection to panel (connection, handler, reconnect)
│   ├── backup/             # Backup/restore (local, s3-compatible, sftp)
│   ├── install/            # Self-install logic (systemd, selfcopy, docker check)
│   └── config/             # Agent config management
│
├── panel/                  # Go panel binary
│   ├── main.go
│   ├── go.mod
│   ├── api/                # REST API handlers
│   ├── nodes/              # Agent connection management
│   ├── db/                 # Database + migrations + models
│   ├── tunnel/             # gRPC server for agent connections
│   ├── auth/               # JWT + middleware
│   └── web/                # SvelteKit SPA frontend
│       └── src/
│           ├── routes/     # Pages (dashboard, nodes, servers, settings)
│           ├── lib/
│           │   ├── components/
│           │   ├── stores/     # Svelte reactive stores
│           │   └── api/        # Fetch wrappers for Go API
│           └── app.html
│
├── proto/
│   └── agent.proto         # gRPC definitions
│
├── scripts/
│   ├── install-agent.sh    # curl | sh one-liner for agent
│   └── install-panel.sh    # curl | sh one-liner for panel
│
├── templates/              # Game server YAML templates
├── go.work                 # Go workspace
├── GEMINI.md               # This file — permanent context
├── CURRENT.md              # Current task and progress — update frequently
└── LICENSE                 # AGPL-3.0
```

---

## Agent Install Layout (On Node)

```
/usr/local/bin/deft-agent       # the binary
/etc/deft/agent.json            # config (panel URL, node ID)
/etc/deft/certs/agent.key       # private key (chmod 600)
/etc/deft/certs/agent.crt       # cert signed by panel
/var/lib/deft/volumes/          # all game server data lives here
/etc/systemd/system/deft-agent.service
```

---

## Systemd Service

```ini
[Unit]
Description=Deft Node Agent
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/deft-agent serve
Restart=always
RestartSec=5
StartLimitIntervalSec=0
User=root
StandardOutput=journal
StandardError=journal
SyslogIdentifier=deft-agent

[Install]
WantedBy=multi-user.target
```

---

## gRPC Protocol Summary

```protobuf
service AgentService {
  rpc Connect(stream AgentMessage) returns (stream PanelCommand) {}
  rpc StreamLogs(LogRequest) returns (stream LogChunk) {}
}
```

Agent sends: heartbeat, command results, resource stats
Panel sends: create/start/stop/delete/exec/backup commands

---

## Game Template Format

```yaml
name: Minecraft Java Edition
docker_image: eclipse-temurin:21-jre
startup_command: java -Xms{MIN_RAM}M -Xmx{MAX_RAM}M -jar server.jar nogui
console_type: stdin           # stdin or rcon
stop_command: stop
default_ports:
  - 25565/tcp
install_script: |
  curl -o server.jar https://...
  echo "eula=true" > eula.txt
variables:
  - name: MAX_RAM
    label: Maximum RAM (MB)
    default: "2048"
```

---

## Security Rules

- Agent command allowlist is hardcoded — no remote override possible
- No arbitrary host OS command execution ever
- File access strictly within `/var/lib/deft/volumes/`
- Mutual TLS on all panel-agent communication
- Every panel instruction logged locally on node (tamper-evident audit log)
- Self-hosted panel = zero Deft company access to nodes

---

## Build Commands

```bash
# Agent
cd agent && go build -o deft-agent .
cd agent && GOOS=linux GOARCH=amd64 go build -o deft-agent-linux-amd64 .
cd agent && go test ./...

# Panel backend
cd panel && go build -o deft-panel .

# Panel frontend (dev)
cd panel/web && npm run dev

# Panel frontend (production build — outputs to web/dist, embedded by Go)
cd panel/web && npm run build

# Generate gRPC from proto
protoc --go_out=. --go-grpc_out=. proto/agent.proto
```

---

## What NOT To Build Yet

Until Phase 1 agent is complete, do not build:
- Panel web UI
- Backup system
- User management / permissions
- Billing integration
- Windows support
- Mobile app
- Plugin marketplace

See CURRENT.md for exactly what to work on right now.

@CURRENT.md

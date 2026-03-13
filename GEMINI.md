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
- Avoid excessive comments in Go code; let the code speak for itself
- Prefer standard library over third party where reasonable
- Never ignore errors with `_` — always handle explicitly
- Wrap errors with `fmt.Errorf("context: %w", err)`
- Use Zerolog for all logging, never fmt.Println in production code
- **Testing:** In Go, test files are colocated in the same directory as the source, named `file_test.go`
- **Workflow:** Never start a new task from CURRENT.md without explicit user permission

---

## Architecture

```
[Browser]
    ↕ HTTPS + WebSocket
[Panel — Go binary with embedded SvelteKit static frontend]
    ↕ gRPC over mutual TLS (agent connects OUTBOUND)
[Agent — Go binary on node]
    ↕ Unix socket (/var/run/docker.sock)
[Docker Engine]
    ↕
[Game Server Containers]
```

### Agent

- Single static Go binary (`deft`), zero runtime dependencies, no CGO
- Managed by systemd as a background service (`deft.service`)
- Command allowlist is hardcoded — cannot execute arbitrary host OS commands
- File operations strictly scoped to `/var/lib/deft/volumes/`
- Includes a `deft uninstall` command for clean removal

### Installer

- Separate Go binary (`deft-install`)
- Interactive setup: prompts for Language (EN/RO) and Mode (Default/Verbose)
- Automatically handles root elevation (sudo)
- Downloads `deft` agent binary for the correct OS/Arch and installs it

### Internal Tools

- **i18n:** Shared `internal/i18n` package using embedded JSON files in `locales/`.

### Communication

- Protocol: gRPC bidirectional streaming
- Security: mutual TLS — both sides verify certificates
- Direction: agent calls out to panel, never the reverse

---

## Tech Stack

| Layer | Technology |
|---|---|
| Agent backend | Go 1.26+ |
| Panel backend | Go 1.26+ |
| Panel frontend | SvelteKit (Static mode) + Tailwind CSS |
| Agent-Panel protocol | gRPC + Protocol Buffers |
| CLI | Cobra |
| Logging | Zerolog |
| Container management | `github.com/docker/docker/client` |
| Init system | systemd |

### Docker Client Usage

```go
import "github.com/docker/docker/client"

// Create client with version negotiation
cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
```

---

## Repository Structure

```
deft/
├── agent/                  # Go agent binary (deft)
│   ├── main.go
│   ├── cmd/                # serve, uninstall
│   ├── docker/             # client, container, console, stats
│   └── ...
├── installer/              # Go installer binary (deft-install)
│   ├── main.go
│   ├── constants.go
│   └── templates/          # deft.service
├── internal/
│   └── i18n/               # Shared localization (JSON + Embed)
├── panel/                  # Go panel binary
├── proto/                  # gRPC definitions
├── go.work                 # (Ignored) local workspace
└── Makefile                # Unified build system (make agent, make installer, make panel)
```

---

## Agent Install Layout (On Node)

```
/usr/local/bin/deft             # the binary
/etc/deft/                      # config and certs
/var/lib/deft/volumes/          # game server data
/etc/systemd/system/deft.service
```

---

## Security Rules

- Agent command allowlist is hardcoded
- File access strictly within `/var/lib/deft/volumes/`
- Mutual TLS on all communication
- Self-hosted panel = zero Deft company access to nodes

---

## Build Commands

```bash
make agent      # Builds agent to bin/deft
make installer  # Builds installer to bin/deft-install
make panel      # Builds panel to bin/deft-panel
make all        # Builds all
```

---

## What NOT To Build Yet

See CURRENT.md for exactly what to work on right now.

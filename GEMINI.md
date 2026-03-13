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
    ↕ HTTPS + WebSocket (handled by Caddy/Nginx in production)
[Panel — Go binary with embedded SvelteKit static frontend]
    ↕ gRPC over mutual TLS (agent connects OUTBOUND)
[Agent Daemon (deftd) — Go binary on node]
    ↕ Unix socket (/var/run/docker.sock)
[Docker Engine]
    ↕
[Game Server Containers]
```

### Agent (Daemon)

- Single static Go binary (`deftd`), zero runtime dependencies, no CGO
- Managed by systemd as a background service (`deft.service`)
- Command allowlist is hardcoded — cannot execute arbitrary host OS commands
- File operations strictly scoped to `/var/lib/deft/volumes/`

### CLI (Universal Controller)

- Single static Go binary (`deft`)
- Namespaced commands: `deft agent ...` (controls daemon), `deft panel ...` (controls docker container)
- Includes a `deft uninstall` command for clean removal

### Installer

- Separate Go binary (`deft-install`)
- Interactive setup: prompts for Language (EN/RO), Mode, and Components (Agent/Panel)
- Automatically handles root elevation (sudo)
- Panel Installation: Pulls and runs the `deft-panel` Docker image with configurable ports

### Internal Tools

- **i18n:** Shared `internal/i18n` package using embedded JSON files in `locales/`.
- **Proto:** Shared `internal/proto` package for gRPC definitions.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Agent backend | Go 1.26+ |
| Panel backend | Go 1.26+ |
| Panel frontend | Svelte 5 + SvelteKit (Static mode) + Tailwind CSS |
| Agent-Panel protocol | gRPC + Protocol Buffers |
| CLI | Cobra |
| Logging | Zerolog |
| Container management | `github.com/docker/docker/client` |
| Init system | systemd |

---

## Repository Structure

```
deft/
├── cmd/
│   ├── deft/               # Universal CLI tool
│   ├── deftd/              # Agent daemon
│   ├── deft-install/       # One-time installer
│   └── deft-panel/         # Panel entry point
├── internal/
│   ├── agent/              # Core agent logic (Docker, Tunnel, Stats)
│   ├── panel/              # Core panel logic (DB, API, gRPC Manager)
│   ├── cli/                # Shared CLI command logic
│   ├── i18n/               # Shared localization (JSON + Embed)
│   └── proto/              # Shared gRPC definitions
├── go.mod                  # Single root module
└── Makefile                # Unified build system
```

---

## Security Rules

- Agent command allowlist is hardcoded
- File access strictly within `/var/lib/deft/volumes/`
- Mutual TLS on all communication
- Self-hosted panel = zero Deft company access to nodes

---

## Future Automation Plans

### Magic SSL (Caddy Integration)
To hit the "North Star" of ease of use, future versions of the installer will offer:
1.  **Automatic Reverse Proxy:** Optionally install Caddy alongside the Panel container.
2.  **Automatic HTTPS:** Use Caddy's built-in Let's Encrypt integration to provide instant SSL for the domain name.
3.  **Docker Compose:** Switch the Panel setup to a tiny `docker-compose.yaml` that orchestrates `deft-panel` and `caddy` together.

---

## Build Commands

```bash
make agent-cli  # Builds CLI to bin/deft
make daemon     # Builds agent daemon to bin/deftd
make installer  # Builds installer to bin/deft-install
make panel      # Builds panel to bin/deft-panel
make docker-panel # Builds local Docker image for the panel (needs sudo)
make all        # Builds all
```

---

## What NOT To Build Yet

See CURRENT.md for exactly what to work on right now.

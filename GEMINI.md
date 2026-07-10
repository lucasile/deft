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
- Intended model is one agent process per machine/node. Multiple agents on one host are only for dev/testing and must use unique `node_id` values and config files.
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
| Panel UI/forms | shadcn-style local components + Superforms SPA mode + Zod |
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
- Production panel startup must fail if gRPC TLS credentials cannot load. Insecure gRPC is only allowed with explicit `DEFT_DEV=true`.
- In dev mode, panel listeners must default to loopback only (`127.0.0.1`). Requiring explicit `DEFT_HTTP_HOST`/`DEFT_GRPC_HOST` overrides prevents accidental insecure exposure.
- Self-hosted panel = zero Deft company access to nodes
- SvelteKit is a static SPA in this repo. Do not put trusted production API/security logic in SvelteKit server routes unless the deployment architecture changes.
- Browser/UI requests go to the Go REST API. The Go panel validates requests, enforces auth/authorization, records state, and sends typed gRPC `PanelCommand` messages to agents.
- Superforms is allowed for panel form UX and client-side validation. Treat it as convenience only; the Go API remains the security boundary and must validate every mutation.
- Never add a generic "run command" path from panel to agent. Add explicit protobuf messages and handler cases for each allowed operation.
- The panel must not allow two live gRPC streams with the same `node_id`; duplicate active IDs are rejected.
- Panel REST APIs require authentication. First-user registration is allowed only while the users table is empty; that user becomes `admin`.
- Passwords are stored as bcrypt hashes. Never store or log plaintext passwords.
- Browser auth uses SQLite-backed sessions in an HttpOnly `deft_session` cookie. Store only session token hashes in the database. Do not use localStorage for auth tokens.
- Authenticated mutation endpoints require `X-CSRF-Token`. The token is session-bound and available from login response or `GET /api/auth/csrf`.
- Auth and mutation endpoints have in-memory per-IP rate limits. Keep this as a baseline; use persistent/distributed limits only if panel deployment becomes multi-instance.
- Audit security-sensitive actions to SQLite `audit_logs`, including auth attempts and container mutations. Do not log plaintext passwords, session tokens, or CSRF tokens.
- Container actions must create `commands` rows before dispatch. Agent `CommandResult` messages complete those rows and update container status when possible. UI should poll `GET /api/commands/{commandID}` after mutations.
- API handlers must limit JSON body size, reject unknown fields, and validate node IDs, command IDs, container names/IDs, and Docker image references before dispatching gRPC commands.
- SQLite access must stay serialized for the single-process panel: one open DB connection, WAL mode, and a busy timeout. Command results must not be lost to transient `SQLITE_BUSY` errors.

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

## Local Development

- Use `scripts/setup-test-auth.sh` to create local mTLS certs and `/etc/deft/agent.json`.
- Use `scripts/run-dev-backend.sh` for local panel development. It sets `DEFT_DEV=true`, starts the Go REST API/UI on `127.0.0.1:3000`, and starts gRPC on `127.0.0.1:50051`.
- `DEFT_DEV=true` does not move API logic into SvelteKit. SvelteKit remains a static SPA; Go remains the API/security/orchestration layer.

---

## What NOT To Build Yet

See CURRENT.md for exactly what to work on right now.

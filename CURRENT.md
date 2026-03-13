# Current Task

## Status: Phase 1 — Agent MVP

## What's Done
- [ ] Nothing yet — project just initialized

## Current Task
**Implement `agent/cmd/install.go`**

The install command should:
1. Check it's running as root (exit with clear error if not)
2. Check Docker is installed (and optionally install it if missing)
3. Find own binary path via `os.Executable()`
4. Copy binary to `/usr/local/bin/deft-agent`
5. Create config directory `/etc/deft/` and data directory `/var/lib/deft/volumes/`
6. Write systemd service file to `/etc/systemd/system/deft-agent.service`
7. Run `systemctl daemon-reload`
8. Run `systemctl enable deft-agent`
9. Run `systemctl start deft-agent`
10. Print success message with next steps

## Next Tasks (do not start yet)
1. `agent/cmd/uninstall.go` — reverse of install, clean removal
2. `agent/docker/client.go` — connect to Docker socket
3. `agent/docker/container.go` — create/start/stop/delete containers
4. `agent/docker/console.go` — stream container stdout/stderr
5. `agent/docker/stats.go` — resource usage per container
6. `agent/tunnel/connection.go` — outbound gRPC to panel
7. `agent/tunnel/handler.go` — dispatch incoming commands

## Blockers / Notes
- None yet

## Last Updated
Project start

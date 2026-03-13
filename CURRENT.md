# Current Task

## Status: Phase 1 — Agent MVP

## What's Done
- [x] Implement `agent` and `installer` separation — clean division between setup tool and service.
- [x] Implement `installer` — interactive (lang/mode), sudo elevation, agent download/install, systemd setup.
- [x] Implement `agent` — `serve` and `uninstall` commands.
- [x] Root-level `Makefile` and `go.work` — unified build system for all modules.
- [x] Support for Go 1.26.1 and i18n (EN/RO).

## Current Task
**Implement `agent/docker/client.go`**

The Docker client should:
1. Initialize a connection to the Docker daemon via the Unix socket (`/var/run/docker.sock`).
2. Provide a wrapper to check the connection status.
3. Handle API version negotiation.

## Next Tasks (do not start yet)
1. `agent/docker/container.go` — create/start/stop/delete containers
2. `agent/docker/console.go` — stream container stdout/stderr
3. `agent/docker/stats.go` — resource usage per container
4. `agent/tunnel/connection.go` — outbound gRPC to panel
5. `agent/tunnel/handler.go` — dispatch incoming commands

## Blockers / Notes
- None yet

## Last Updated
Project start

# Current Task

## Status: Phase 1 — Agent MVP

## What's Done
- [x] Implement `agent` and `installer` separation — clean division between setup tool and service.
- [x] Implement `installer` — interactive (lang/mode), sudo elevation, agent download/install, systemd setup.
- [x] Implement `agent` — `serve` and `uninstall` commands.
- [x] Root-level `Makefile` and `go.work` — unified build system for all modules.
- [x] Support for Go 1.26.1 and i18n (EN/RO).
- [x] Implement `agent/docker/client.go` — Docker daemon connection and version negotiation.

## Current Task
**Implement `agent/docker/container.go`**

The container management should:
1. Implement a method to create a container from a given image and configuration.
2. Implement methods to start, stop, and delete containers.
3. Handle error cases gracefully (e.g., container already exists/stopped).

## Next Tasks (do not start yet)
1. `agent/docker/console.go` — stream container stdout/stderr
2. `agent/docker/stats.go` — resource usage per container
3. `agent/tunnel/connection.go` — outbound gRPC to panel
4. `agent/tunnel/handler.go` — dispatch incoming commands

## Blockers / Notes
- None yet

## Last Updated
Project start

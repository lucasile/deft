# Current Task

## Status: Phase 1 — Agent MVP

## What's Done
- [x] Implement `agent` and `installer` separation — clean division between setup tool and service.
- [x] Implement `installer` — interactive (lang/mode), sudo elevation, agent download/install, systemd setup.
- [x] Implement `agent` — `serve` and `uninstall` commands.
- [x] Root-level `Makefile` and `go.work` — unified build system for all modules.
- [x] Support for Go 1.26.1 and i18n (EN/RO).
- [x] Implement `agent/docker/client.go` — Docker daemon connection and version negotiation.
- [x] Implement `agent/docker/container.go` — Container lifecycle (create, start, stop, remove).
- [x] Implement `agent/docker/console.go` — Generic log streaming and command stdin.

## Current Task
**Implement `agent/docker/stats.go`**

The stats management should:
1. Implement a method to stream real-time resource usage (CPU, Memory, Network, Disk I/O) for a specific container.
2. Return a channel or stream that the agent can eventually send back to the panel.

## Next Tasks (do not start yet)
1. `agent/tunnel/connection.go` — outbound gRPC to panel
2. `agent/tunnel/handler.go` — dispatch incoming commands

## Blockers / Notes
- None yet

## Last Updated
Project start

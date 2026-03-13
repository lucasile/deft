# Current Task

## Status: Phase 1 — Agent MVP

## What's Done
- [x] Implement `agent` and `installer` separation — clean division between setup tool and service.
- [x] Implement `installer` — interactive (lang/mode), sudo elevation, agent download/install, systemd setup.
- [x] Implement `agent` — `serve` and `uninstall` commands.
- [x] Root-level `Makefile` and `go.work` — unified build system for all modules.
- [x] Support for Go 1.26.1 and i18n (EN/RO).
- [x] Implement `agent/docker/client.go` — Docker daemon connection and version negotiation.
- [x] Implement `agent/docker/container/` — Container lifecycle (create, start, stop, remove).
- [x] Implement `agent/docker/console/` — Generic log streaming and command stdin.
- [x] Implement `agent/docker/stats/` — Real-time resource usage streaming.

## Current Task
**Implement `agent/tunnel/connection.go`**

The tunnel connection should:
1. Establish a bidirectional gRPC stream to the central panel.
2. Handle mutual TLS (mTLS) authentication using certificates from `/etc/deft/certs/`.
3. Implement an exponential backoff reconnection strategy (1s to 60s).

## Next Tasks (do not start yet)
1. `agent/tunnel/handler.go` — dispatch incoming commands

## Blockers / Notes
- None yet

## Last Updated
Project start

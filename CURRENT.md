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
- [x] Implement `agent/tunnel/connection.go` — Bidirectional gRPC stream with mTLS and backoff.
- [x] Implement `agent/tunnel/handler.go` — Command dispatching and result reporting.
- [x] Wire everything together in the `serve` command.

## Next Tasks (do not start yet)
1. Phase 2: Panel MVP (Node.js/SvelteKit)
2. Agent Join Token flow (Bootstrap API)

## Blockers / Notes
- None yet

## Last Updated
Project start

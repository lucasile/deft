# Current Task

## Status: Phase 2 — Panel MVP

## What's Done
- [x] Implement `agent` and `installer` separation — clean division between setup tool and service.
- [x] Implement `installer` — interactive (lang/mode), sudo elevation, agent download/install, systemd setup.
- [x] Implement `agent` — `serve`, `uninstall`, and `service` commands.
- [x] Root-level `Makefile` and `go.work` — unified build system for all modules.
- [x] Support for Go 1.26.1 and i18n (EN/RO).
- [x] Implement `agent/docker/client.go` — Docker daemon connection and version negotiation.
- [x] Implement `agent/docker/container/` — Container lifecycle (create, start, stop, remove).
- [x] Implement `agent/docker/console/` — Generic log streaming and command stdin.
- [x] Implement `agent/docker/stats/` — Real-time resource usage streaming.
- [x] Implement `agent/tunnel/connection.go` — Bidirectional gRPC stream with mTLS and backoff.
- [x] Implement `agent/tunnel/handler.go` — Command dispatching and result reporting.
- [x] Wire everything together in the `serve` command.
- [x] Initial Panel Setup — Go backend with embedded SvelteKit static frontend.
- [x] Modular Panel Backend — Reorganized into `internal/` (db, api, nodes) with `schema.sql`.

## Current Task
**Implement Panel gRPC Server & REST API Core**

The panel should:
1. Initialize a gRPC server to accept connections from agents.
2. Implement the `Connect` method from `agent.proto`.
3. Set up a simple REST API to trigger container actions (Create, Start, etc.) for testing.
4. Integrate SQLite database for basic node/container tracking.

## Next Tasks (do not start yet)
1. User Authentication (JWT).
2. SvelteKit UI Development (Dashboard, Node Management, Console).
3. Agent Join Token flow (Bootstrap API).


## Blockers / Notes
- None yet

## Last Updated
Project start

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
- [x] Fixed current `go test ./...` blocker in installer (`errors.New` instead of dynamic `fmt.Errorf`).
- [x] Confirmed frontend architecture — SvelteKit builds as a static SPA embedded in the Go panel binary.
- [x] Added initial Go REST endpoints for node listing and typed container actions that dispatch over gRPC.
- [x] Locked gRPC TLS behavior — production fails if TLS credentials are missing; insecure gRPC requires explicit `DEFT_DEV=true`.
- [x] Added first-user auth foundation — users table, bcrypt password hashes, SQLite-backed sessions, HttpOnly cookies, and admin-only API middleware.
- [x] Added CSRF protection for authenticated mutation endpoints.
- [x] Added in-memory rate limiting for auth and mutation endpoints.
- [x] Added SQLite audit logs for auth and container mutation attempts.
- [x] Added command result tracking — commands are recorded as pending, completed from agent `CommandResult`, and exposed via `GET /api/commands/{commandID}`.
- [x] Hardened API input parsing — body size limits, unknown JSON field rejection, and validation for node IDs, command IDs, container names/IDs, and image references.
- [x] Built initial Svelte dashboard — node list, create/start/stop/remove controls, command status polling, and cookie/CSRF API client wiring.
- [x] Adopted shadcn-style local Svelte UI components and Superforms SPA-mode validation for panel forms.

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
- SvelteKit is UI-only for this architecture. Production API/security logic belongs in the Go panel unless the deployment model changes.
- Frontend forms may use Superforms for UX/client validation, but Go API validation remains the security boundary.
- Browser/UI -> Go REST API -> Go node manager -> gRPC stream -> agent -> Docker.
- Keep panel-to-agent operations typed and allowlisted through protobuf. Do not add arbitrary host command execution.
- One installed agent per machine is the intended model. Dev/test multi-agent runs must use unique `node_id` values.
- Production gRPC must use mTLS. Insecure gRPC is only for local development with `DEFT_DEV=true`.
- Dashboard is functional for the current backend core. It uses local shadcn-style components under `internal/panel/ui/web/src/lib/components/ui`. It still needs container listing, clearer empty/setup states, and production UI polish.

## Last Updated
2026-07-09

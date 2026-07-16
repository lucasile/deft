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
- [x] Added authenticated Server-Sent Events for panel updates, starting with node changes and command completion invalidation.
- [x] Added backend foundation for panel-local agent join flow: one-time join tokens, CSR signing, joined node cert metadata, and production cert/node identity checks.
- [x] Added installer-driven agent join: installer can use either browser approval link flow or headless join token flow, generate local agent key/CSR, write certs/config, and start systemd service.
- [x] Added join token manager foundation: list recent tokens, show active/used/expired/revoked state, and revoke unused tokens.
- [x] Added local multi-agent dev helper: `make dev-agent NAME=<name> [TOKEN=<token>]` creates a clean isolated `.deft/dev-agents/<name>` config/certs directory and runs `deftd` in the foreground. If `TOKEN` is omitted, it creates a browser approval link against `PANEL_URL` or `http://localhost:3000`.
- [x] Scoped container management to Deft-owned containers: created containers get `deft.managed=true` labels, agents sync only labeled Docker containers, and panel inventory reconciles those rows.
- [x] Decoupled user-facing container/server names from Docker names. Panel generates Deft-owned Docker names (`deft-...`) and stores the friendly name in labels/inventory.
- [x] Added live container logs on the container detail page. The browser requests a short-lived stream ID with a CSRF-protected POST, then opens an authenticated SSE stream; the panel sends typed gRPC follow/cancel commands to the agent and cancels the Docker log follow when the browser disconnects.
- [x] Added basic container configuration at create time: port mappings, environment variables, volume mounts, and restart policy. Volume host paths are intentionally restricted to `/var/lib/deft/volumes/...`.
- [x] Moved the expanded container create form to `/nodes/{nodeID}/containers/new` so the node page stays focused on agent status and the container list.
- [x] Added the first server abstraction foundation: `servers` table, server manager, `GET /api/servers`, `GET /api/servers/{serverID}`, create-time server records, and inventory-based linking from server resource IDs to real Docker container IDs.
- [x] Made servers visible as the dashboard's primary list and added `/servers/{serverID}` for read-only server overview, desired config, node link, and linked container link.
- [x] Added dashboard-first server creation UX: the dashboard has a visible `Create server` action, `/servers/new` lets users choose an online node when needed, and the node create flow now presents itself as server creation while still using container-backed implementation under the hood.
- [x] Added history-aware back navigation for nested panel routes so shared entry points can return to the actual previous page while still falling back to the dashboard or parent route on direct loads.
- [x] Server creation now gives visible in-flight feedback and redirects to the new server detail page. The create response returns `server_id` alongside `command_id`.
- [x] Server detail pages now subscribe to panel events and refresh quietly, so create/start/stop status changes do not require manual page refreshes.

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
4. Node lifecycle controls: remove/archive offline nodes, disable/revoke joined nodes, and hide archived nodes from the default dashboard.


## Blockers / Notes
- SvelteKit is UI-only for this architecture. Production API/security logic belongs in the Go panel unless the deployment model changes.
- Frontend forms may use Superforms for UX/client validation, but Go API validation remains the security boundary.
- Browser/UI -> Go REST API -> Go node manager -> gRPC stream -> agent -> Docker.
- Browser live updates should use authenticated SSE invalidation events before adding polling. Use WebSockets only when the browser needs bidirectional streams such as console input.
- Live log streams are read-only SSE. Starting a stream must remain CSRF-protected; do not let unauthenticated or plain cross-site GET requests trigger agent actions.
- Keep panel-to-agent operations typed and allowlisted through protobuf. Do not add arbitrary host command execution.
- Deft should manage labeled containers by default. Do not treat every Docker container on the host as managed; unmanaged discovery should be explicit and read-only at first.
- User-facing names are display names, not Docker container names. Docker names should be generated and treated as implementation details.
- Servers are the intended product object. Containers are implementation detail/debug surface. New game, WireGuard, backups, and settings work should attach to servers first.
- Dashboard entry points should say `server`, not `container`. Container routes may remain as internal/debug implementation paths until the server abstraction fully owns detail and action pages.
- Successful server creation should land on the server detail page, not the node/container debug page.
- Server detail should become the main management surface for actions, logs, config, health, and backups. Raw container pages can remain available as advanced/debug views, but should not be the normal user path.
- Server detail pages should live-update from panel events anywhere status or linked container state can change.
- Avoid duplicate primary CTAs in the same card empty state. If a card header already has the action, the empty state should explain what is missing, not repeat the same button.
- Do not allow arbitrary host path mounts from the panel. Deft-created volume mounts should stay under `/var/lib/deft/volumes/...` unless a future explicit admin-only escape hatch is designed and audited.
- One installed agent per machine is the intended model. Dev/test multi-agent runs must use unique `node_id` values.
- Production gRPC must use mTLS. Insecure gRPC is only for local development with `DEFT_DEV=true`.
- Deft has no global panel authority. Each self-hosted panel is its own trust root and issues its own agent join tokens/certificates.
- Production agent certs must include `deft:node:<node_id>` identity and match the stored node certificate fingerprint.
- Agent private keys must be generated locally by the agent installer and never sent to the panel. Only CSRs are sent during join.
- Keep both agent join UX paths: browser approval link for interactive installs, join token for headless automation.
- Join tokens are short-lived and single-use; admins must be able to list recent token metadata and revoke unused tokens.
- Join token manager displays at most 5 tokens. Revoked tokens remain visible briefly, then age out of the panel list.
- Uninstalled agents should remain visible as offline nodes until an admin explicitly removes, archives, or disables them. Removal/revocation must be audited.
- Dashboard is functional for the current backend core. It uses local shadcn-style components under `internal/panel/ui/web/src/lib/components/ui`. It still needs container listing, clearer empty/setup states, and production UI polish.

## Last Updated
2026-07-15

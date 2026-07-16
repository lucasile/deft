# deft

**Server management that gets out of your way.**

Deft is an open source panel for managing game servers and Docker applications across multiple nodes. Install the agent on any Linux server with one command — it connects back to your panel automatically. No open ports, no manual configuration, no 48-hour setup guides.

---

## How it works

Deft has two parts: a **panel** you run once, and a lightweight **agent** you drop on each server node.

```
panel.yourdomain.com
    ↕ encrypted tunnel
your-game-server-1          # agent installed, zero inbound ports
your-game-server-2          # agent installed, zero inbound ports
your-game-server-3          # agent installed, zero inbound ports
```

The agent connects *outward* to the panel — not the other way around. This means your game server nodes need no open inbound ports and work behind NAT out of the box.

---

## Features

- **Zero inbound ports** — agents connect outward, works behind any firewall or NAT
- **One-command install** — panel and agent both set up in under 5 minutes
- **Game servers and apps** — run Minecraft, Rust, CS2, or any Docker image side by side
- **Live console** — real-time server output and command input in the browser
- **File manager** — browse, edit, upload files without SSH
- **Backups** — scheduled or on-demand, to local disk or S3-compatible storage
- **Resource limits** — hard CPU, RAM, and disk limits per server via Docker cgroups
- **Game templates** — community-maintained configs for 50+ games, one-click install
- **Self-hosted or cloud** — run your own panel or use ours

---

## Self-hosting

Deft is designed to run entirely on your own infrastructure. The panel requires a single Linux VPS. Nodes can be any Linux server — bare metal, VPS, home server, or cloud VM.

---

## Architecture

```
agent/          Go binary — node daemon, Docker management, gRPC tunnel
panel/          Go binary — REST API, node orchestration, embedded frontend
panel/web/      SvelteKit SPA — the browser interface
proto/          Protocol Buffer definitions for agent-panel communication
templates/      YAML game server templates
scripts/        Install scripts
```

The panel and agent are both single static binaries with no runtime dependencies. The panel embeds the frontend — deploying Deft is copying one file and running it.

---


## License

[AGPL-3.0](LICENSE) — free to self-host forever. If you modify Deft and run it as a hosted service, you must publish your modifications under the same license.

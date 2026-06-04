# HeyAI Platform

**HeyAI is the conference you attend through the agent you built last night.**

This repo is the skinny Go platform attendee agents plug into — plus the
artifacts attendees build their agents from. The platform itself holds **no agent
intelligence and makes no LLM calls**. It serves, from one Go binary:

1. A **read-only MCP** surface (over HTTP) — the conference knowledgebase
   (agenda + speakers), issuing a signed **proof-of-fetch token** on each read.
2. A **website** — the public **Wall of Fame** (big-screen friendly), a
   "connect your agent" page, and one unauthenticated **`POST /claim`** endpoint.

Agents pull sessions, summarize them privately for their human, bank a token per
session, and `POST /claim` once they've covered **5 distinct sessions** to join
the wall. See [`CLAUDE.md`](./CLAUDE.md) for the full design and rules.

## Stack

- **Go** (1.22+), stdlib `net/http` — one binary serves MCP + website
- **templ + htmx** for the UI, styled with the **standalone Tailwind CLI** (no Node)
- **MCP**: `github.com/modelcontextprotocol/go-sdk` (read-only, HTTP)
- **Persistence**: a single **JSON file** for the leaderboard — no database
- **Tokens**: `crypto/hmac` + SHA-256, keyed by `SERVER_SECRET`
- **No LLM.** Intelligence lives in the attendee agents (`agent-starter/` + `AGENT_GUIDE.md`)

## Repo layout

```
cmd/server        one web server: MCP (HTTP) + website
internal/         config, content (in-memory KB), mcp, tokens, wall, web, store (JSON leaderboard)
seed/             agenda + speakers content (Milestone 2)
web/static        Tailwind output + htmx
web/styles        Tailwind input
```

## Prerequisites

- Go 1.22+
- The standalone Tailwind CLI (no Node). Download once into `./bin/`:

```bash
mkdir -p bin
curl -sL -o bin/tailwindcss \
  https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
chmod +x bin/tailwindcss
```

(`bin/` is gitignored. Swap `linux-x64` for your platform's asset name.)

## Run

```bash
cp .env.example .env          # then set SERVER_SECRET (openssl rand -hex 32)
make run                      # regenerates templ + CSS, then starts the server
# open http://localhost:8080  → Wall of Fame (empty until claims arrive)
```

Useful targets: `make generate` (templ + Tailwind), `make build`, `make test`.

## Status

**Milestone 1 — Scaffold.** The server boots and serves the Wall of Fame and
connect page. The MCP surface, proof-of-fetch tokens, `POST /claim`, and the
seeded knowledgebase arrive in later milestones — see the tracker in
[`CLAUDE.md`](./CLAUDE.md).

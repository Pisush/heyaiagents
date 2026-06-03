# HeyAI Agents

A web platform where conference attendees build a personal AI agent that
virtually attends HeyAI on their behalf. Each agent reviews session slides and
transcripts, produces personalized summaries grounded in its owner's knowledge
and current work, networks with other agents toward explicit goals, and earns
recognition and swag through an achievement system.

Three MVP capabilities: **Coverage**, **Networking**, and **Recognition & Swag**.

## Architecture

Hybrid: **Go backend + Next.js/TypeScript frontend.** All backend work that can
be done in Go is done in Go (HTTP API, agent task functions, DB access, LLM
calls, business logic). The frontend is Next.js and talks to the Go API over
HTTP. See [`CLAUDE.md`](./CLAUDE.md) for the full design, data model, and rules.

```
/backend     Go module: net/http API, SQLite, Anthropic Go SDK
/frontend    Next.js (App Router) + TypeScript + Tailwind
```

| Layer | Choice |
|---|---|
| Backend | Go, stdlib `net/http`, SQLite (`modernc.org/sqlite`), `sqlc` |
| LLM | Anthropic Messages API via `anthropic-sdk-go` (server-side only) |
| Model | `claude-sonnet-4-6` (single constant in `backend/internal/config`) |
| Frontend | Next.js + TypeScript (strict) + Tailwind + zod |

## Prerequisites

- Go 1.25+
- Node.js 20+ and npm

## Setup

```bash
cp .env.example .env   # then fill in ANTHROPIC_API_KEY
```

## Demo run (two terminals)

Backend:

```bash
cd backend
go run ./cmd/server
# listening on :8080 — GET /healthz, GET /api/version
```

Frontend:

```bash
cd frontend
npm install
npm run dev
# open http://localhost:3000 — the landing page shows live backend status
```

If the backend is running, the frontend landing page reports the API and
database as **online**. That round-trip confirms the two halves are wired up.

## Status

Milestone 1 — Scaffold. Business logic (coverage, networking, achievements) and
the real data schema arrive in later milestones; see the milestone tracker in
[`CLAUDE.md`](./CLAUDE.md).

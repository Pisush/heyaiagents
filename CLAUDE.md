# CLAUDE.md — HeyAI Agents
This file is read by Claude Code at the start of every session. Keep it current as the project evolves. Update it when a milestone is completed or an open decision is resolved.
---
## What we're building
A web platform where conference attendees build a personal AI agent that virtually attends HeyAI on their behalf. The agent reviews session slides and transcripts, produces personalized summaries grounded in its owner's knowledge and current work, networks with other agents toward explicit goals, and earns recognition and swag through an achievement system.
Three capabilities, all in MVP scope: **Coverage**, **Networking**, **Recognition & Swag**.
---
## Architecture
**Hybrid: Go backend + Next.js/TS frontend.** All backend work that can be done in Go is done
in Go (HTTP API, agent task functions, DB access, LLM calls, business logic). The frontend is
Next.js/TypeScript and talks to the Go API over HTTP. The frontend's own route handlers stay a
thin BFF — no agent logic or LLM calls there.

### Backend stack (Go)
| Layer | Choice |
|---|---|
| Language | Go strict — no `interface{}`/`any` without a justifying comment |
| HTTP | Go stdlib `net/http` with the 1.22+ `ServeMux` routing (no router dep unless needed) |
| DB | SQLite via `modernc.org/sqlite` (pure Go, no cgo). Plain SQL migrations kept Postgres-compatible for later swap; query layer via `sqlc` |
| LLM | Anthropic Messages API via `anthropic-sdk-go` (official Go SDK), server-side only |
| Model | `claude-sonnet-4-6` — single constant `ModelName` in `internal/config`, never scattered literals |
| Validation | Go struct + decode-time validation at the LLM boundary, always |

### Frontend stack (TypeScript)
| Layer | Choice |
|---|---|
| Language | TypeScript strict — no `any` without a justifying comment |
| Framework | Next.js (App Router) — UI + thin BFF route handlers only |
| Validation | zod — all API responses validated at the client boundary |
| Model | `claude-sonnet-4-6` — single mirrored constant in `lib/config.ts` if the UI needs it |
| Styling | Tailwind CSS |
---
## Repo layout (establish at scaffold, maintain)
```
/backend                 Go module (heyai-agents backend)
  go.mod
  cmd/server/main.go     HTTP entrypoint
  internal/
    config/              config.go — ModelName + global constants
    api/                 HTTP handlers + router wiring
    agents/              Agent task functions (coverage, networking)
    prompts/             All prompts as Go modules — never inline strings
    db/                  sqlite client, sqlc-generated code, data access helpers
    llm/                 Anthropic Go SDK client wrapper (server-side only)
  migrations/            Plain SQL migrations (Postgres-compatible)
  seed/                  Seed program + seed data (HeyAI 2026 sessions, demo users)
  sqlc.yaml
/frontend                Next.js (App Router) + TS + Tailwind
  app/                   pages + thin BFF route handlers
  components/            shared UI components
  lib/                   frontend config (API base URL), fetch helpers
  types/                 shared TS types + zod schemas for API responses
.env.example
README.md
```
---
## Data model (key entities)
- **User** — id, name, email, role
- **AgentProfile** — userId, expertise[], pastWork, currentFocus, networkingGoals[], offers[], seeks[], voice
- **OpenSourceProject** — ownerUserId, name, description, needsHelpWith, repoUrl
- **Conference / Session / SessionContent** — schedule; slides text (primary) + transcript text (optional)
- **Summary** — userId, sessionId, structured output (takeaways, relevanceToCurrentWork, actionItems, seeInPerson)
- **DailyDigest** — userId, day, structured summary, sourceSummaryIds
- **MatchProposal** — fromUserId, toUserId, goalType (`linkedin` | `oss_seek_contributors` | `oss_offer_help`), reason, draftMessage, status
- **Achievement** — key, label, description, criteria
- **AchievementUnlock** — userId, achievementKey, unlockedAt, evidence
Wall of Fame = a query over AchievementUnlock + User, not a separate table.
There is a reserved seam for a future **ParticipationAction** entity. Do not create it yet.
---
## Agent task rules
- Every agent task is a **pure-ish Go function** in `backend/internal/agents/`: `(inputs) → validated struct`. Runnable, re-runnable, testable.
- Coverage: `(AgentProfile, SessionContent) → Summary`. Summaries must explicitly address relevance to `currentFocus`.
- Networking: `(thisProfile, otherProfiles[], goals) → MatchProposal[]`. Goal-typed, mutual-value aware.
- All LLM calls **server-side only**, in the Go backend — never in the frontend.
- All prompts live in `backend/internal/prompts/`. No inline prompt strings elsewhere.
- All LLM outputs validated by Go boundary validation before use; the frontend re-validates API responses with zod schemas in `frontend/types/`.
- Runs are **idempotent and cached** — re-running coverage returns the stored result unless the profile or content changed. Log token usage per run.
---
## Approval gates (what requires human one-tap approval before firing)
- Sending a LinkedIn connection request
- Offering to contribute to someone's OSS project
- Any outbound action that reaches a real person or commits the human to something
Discovery, match scoring, and draft messages are automatic. Sending is not.
---
## Achievement rules (business logic — enforce these in code)
| Achievement | Criteria |
|---|---|
| Wall of Fame | Summarized ≥ 5 **distinct** sessions |
| Networker | ≥ 3 match proposals **accepted** (status = `sent`) |
| Open Source Hero | ≥ 1 OSS match accepted in either direction |
Anti-gaming: summaries must be for distinct sessions; sanity-check that summary substance is non-trivial (prompt the model to flag low-effort outputs and don't count them toward achievements).
---
## Hard rules (never violate)
- No secrets in client code or committed files. `ANTHROPIC_API_KEY` lives in `.env` only.
- No raw model JSON trusted without Go boundary validation (frontend re-validates with zod).
- No inline prompt strings outside `backend/internal/prompts/`.
- Do not implement **live participation** (agents speaking/voting in sessions) — not in scope, leave seams only.
- Do not implement real-time audio ingestion — transcripts arrive as text.
- Do not build real agent-to-agent messaging protocols — networking is simulated via stored profiles.
---
## Resolved decisions
- [x] Architecture: **hybrid** — Go backend (prioritized for all backend work) + Next.js/TS frontend.
- [x] Repo/package name: `heyai-agents`.

## Open decisions (do not silently resolve — ask the human)
- [ ] Auth: real or mock? (current default: mock/seeded)
- [ ] Swag: virtual badges only, or also physical swag codes at the event?
- [ ] Wall of Fame: public to all attendees, or opt-in?
- [ ] Networking consent: does opt-in required before profile is visible to other agents for matching? (strong default: yes)
- [ ] LinkedIn: human-approved send (default) or fully autonomous?
---
## Milestone tracker (update as milestones complete)
- [x] 1 — Scaffold: Go backend (net/http + SQLite + Anthropic Go SDK) + Next.js/TS/Tailwind frontend + `.env.example` + README
- [ ] 2 — Data model + seed: schema, migrations, seed script (HeyAI 2026, 6–8 sessions, 2 demo users, OSS projects)
- [ ] 3 — Build-your-agent: sign-up + agent profile builder (editable)
- [ ] 4 — Capability A: coverage run, summaries grounded in agent profile + currentFocus, daily digest
- [ ] 5 — Capability B: goal-typed match generation, "People to meet" UI, human-approval on outbound actions
- [ ] 6 — Capability C: achievements, Wall of Fame/leaderboard, badge/swag display, anti-gaming
- [ ] 7 — Polish: empty/loading/error states, token-usage display, README demo script
**Current milestone: 2 — Data model + seed (pending human go-ahead).**
Pause and check in with the human at the end of each milestone before starting the next.

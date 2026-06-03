# CLAUDE.md — HeyAI Agents
This file is read by Claude Code at the start of every session. Keep it current as the project evolves. Update it when a milestone is completed or an open decision is resolved.
---
## What we're building
A web platform where conference attendees build a personal AI agent that virtually attends HeyAI on their behalf. The agent reviews session slides and transcripts, produces personalized summaries grounded in its owner's knowledge and current work, networks with other agents toward explicit goals, and earns recognition and swag through an achievement system.
Three capabilities, all in MVP scope: **Coverage**, **Networking**, **Recognition & Swag**.
---
## Stack
| Layer | Choice |
|---|---|
| Language | TypeScript strict — no `any` without a justifying comment |
| Framework | Next.js (App Router) — UI + API routes in one repo |
| DB | SQLite via Prisma (Postgres-compatible schema for later swap) |
| LLM | Anthropic Messages API via `@anthropic-ai/sdk`, server-side only |
| Model | `claude-sonnet-4-6` — single constant in `lib/config.ts`, never scattered literals |
| Validation | zod — all LLM output validated at the boundary, always |
| Styling | Tailwind CSS |
---
## Repo layout (establish at scaffold, maintain)
```
/app              Next.js App Router pages and API routes
/components       Shared UI components
/lib
  /agents         Agent task functions (coverage, networking)
  /prompts        All prompts as dedicated modules — never inline strings
  /db             Prisma client, data access helpers
  /config.ts      MODEL_NAME and other global constants
/prisma
  schema.prisma
  /seed           Seed script + seed data (HeyAI 2026 sessions, demo users)
/types            Shared TypeScript types + zod schemas for agent outputs
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
- Every agent task is a **pure-ish server-side function**: `(inputs) → validated structured output`. Runnable, re-runnable, testable.
- Coverage: `(AgentProfile, SessionContent) → Summary`. Summaries must explicitly address relevance to `currentFocus`.
- Networking: `(thisProfile, otherProfiles[], goals) → MatchProposal[]`. Goal-typed, mutual-value aware.
- All LLM calls **server-side only** — never in client components or browser-exposed routes.
- All prompts live in `/lib/prompts/`. No inline prompt strings elsewhere.
- All LLM outputs validated with zod schemas defined in `/types/`.
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
- No raw model JSON trusted without zod validation.
- No inline prompt strings outside `/lib/prompts/`.
- Do not implement **live participation** (agents speaking/voting in sessions) — not in scope, leave seams only.
- Do not implement real-time audio ingestion — transcripts arrive as text.
- Do not build real agent-to-agent messaging protocols — networking is simulated via stored profiles.
---
## Open decisions (do not silently resolve — ask the human)
- [ ] Auth: real or mock? (current default: mock/seeded)
- [ ] Repo/package name (suggestion: `heyai-agents`)
- [ ] Swag: virtual badges only, or also physical swag codes at the event?
- [ ] Wall of Fame: public to all attendees, or opt-in?
- [ ] Networking consent: does opt-in required before profile is visible to other agents for matching? (strong default: yes)
- [ ] LinkedIn: human-approved send (default) or fully autonomous?
---
## Milestone tracker (update as milestones complete)
- [ ] 1 — Scaffold: Next.js + TS + Tailwind + Prisma/SQLite + `.env.example` + README
- [ ] 2 — Data model + seed: schema, migrations, seed script (HeyAI 2026, 6–8 sessions, 2 demo users, OSS projects)
- [ ] 3 — Build-your-agent: sign-up + agent profile builder (editable)
- [ ] 4 — Capability A: coverage run, summaries grounded in agent profile + currentFocus, daily digest
- [ ] 5 — Capability B: goal-typed match generation, "People to meet" UI, human-approval on outbound actions
- [ ] 6 — Capability C: achievements, Wall of Fame/leaderboard, badge/swag display, anti-gaming
- [ ] 7 — Polish: empty/loading/error states, token-usage display, README demo script
**Current milestone: 1 — Scaffold.**
Pause and check in with the human at the end of each milestone before starting the next.

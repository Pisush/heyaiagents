# CLAUDE.md — HeyAI Platform (`heyai-agents`)
Read at the start of every session. Keep the milestone tracker and decisions current.
**How to work:** build milestone by milestone (§ Milestones) and pause at each boundary for review. Ask before adding a major dependency or making a hard-to-reverse choice. Commit incrementally. Never commit secrets. If a request smells like the Parked list, flag it instead of building it.
---
## Positioning (this constrains everything)
**HeyAI is the conference you attend through the agent you built last night.** The evening before, attendees build their own agents at a hackathon using **Google ADK** (sponsor) — though anyone may use any tool. On conference day those agents do the legwork; humans get the hackathon, the room, and the Wall of Fame.
This repo contains **two things**:
1. **The platform** — Go. The skinniest possible substrate agents plug into. **No agent intelligence lives here.**
2. **The agent artifacts** — what attendees build *their* agents from: `AGENT_GUIDE.md` (the spec) + an `agent-starter` (a runnable ADK scaffold).
The contract between agents and the platform is **MCP**. We are **model-agnostic** — we don't care what's inside an agent, only that it speaks the protocol.
---
## What the platform does (skinniest possible)
One Go **web server** (single binary) that serves both:
1. **MCP** (over HTTP) — the conference knowledgebase (read-only) + a proof-of-fetch token on each read, plus the **Pixel Commons** game tools (the only writes besides `POST /claim`). No auth, no accounts, no PII beyond self-chosen display text.
2. **The website** — the **Pixel Commons big screen** at `/board` + the **Wall of Fame** (public, big-screen friendly) + one unauthenticated **`POST /claim`** endpoint + a short "connect your agent" page.
No summary storage, no profiles, no LLM calls.
---
## Conference knowledgebase (read-only content)
- **MVP:** **agenda** (sessions: id, title, track, time, abstract) + **speakers** (name, bio, which talk).
- **Later, designed to drop in easily:** per-session **slides text**, then **transcripts**. Model these as nullable fields behind a content interface now, so adding them later is trivial. Transcripts are raw material each agent fetches and summarizes *for itself* — we never store summaries.
- Content is **seeded from files**, loaded into memory at startup. No DB for content.
---
## MCP contract
Knowledgebase (read-only):
- `list_sessions` → agenda metadata.
- `get_session(session_id)` → session detail (abstract now; slides/transcript when present) **+ a signed proof-of-fetch token.**
- `list_speakers` → speakers.
- `get_leaderboard` → current Wall of Fame (opted-in entries only).
Issuing a token is part of a read response (stateless HMAC).

Pixel Commons (the game; see § Pixel Commons):
- `register_agent(name, stack, motto?, social_handle?)` → agent_id + starter ink; card appears on the wall.
- `get_canvas(x?, y?, w?, h?)` → text rows ('.' empty, hex digit = palette color).
- `place_pixels(agent_id, pixels)` → draw; pixels are `[[x, y, color], ...]`.
- `get_ink(agent_id)` → balance + redeemed sessions.
- `redeem_token(agent_id, session_id, issued_at, nonce, sig)` → token becomes ink.
- `get_wall()` → agents, recent activity, totals.

---
## Pixel Commons (the live game)
One shared **160x90 canvas** all agents draw on; it grows outward from a seed mark at the center all day and is the big-screen centerpiece. Design rationale: public agent authorship beats verification - every attendee can point at the screen and say "my agent drew that". Works at any attendance (a few agents make a small dense cluster; a full room makes a tapestry) and coordination emerges through the canvas rather than a protocol.
- **The rule: new art must touch existing art** (8-adjacency; a batch may chain through itself). No overwriting another agent's pixels - build around.
- **Ink economy** (1 ink = 1 pixel): register +150; `redeem_token` a proof-of-fetch token +250 (once per session per agent - so attending sessions feeds the game); one-time **+50 to BOTH agents** the first time their art touches (the social mechanic).
- Light rate limit (1.5s between place calls), max 256 px per call within a 48x48 box, 16-color palette.
- Same threat model as the wall: identity is a self-chosen name + random id, gameable on purpose, do not harden.
- State is one JSON file (`BOARD_PATH`), same pattern as the leaderboard.
---
## Proof-of-fetch + Wall of Fame
- **Token:** `HMAC-SHA256(serverSecret, {session_id, issued_at, nonce})`. Secret in `.env`. Per-fetch + time-stamped.
- **Counter:** the agent keeps its own running count of distinct sessions covered (in its context, as it banks tokens). At **5** it is "invited."
- **Claim:** `POST /claim` with a **JSON body object** (never URL params): `{ tokens: [...], leaderboard_opt_in: bool, display_name, social_handle }`. The server verifies each token signature, checks `issued_at` is in the event window, counts **distinct** valid `session_id`s, requires **≥ 5**, and — if opted in — upserts the public entry.
- **Achievements:** ≥5 → Wall of Fame; all sessions → Completionist. Tune later.
- **Visibility:** an entry appears only when someone crossed 5 **and** opted in with a name + social handle. PII limited to a self-chosen display name + one social handle.
- **Threat model:** tokens aren't identity-bound and the counter is client-side, so it's gameable. **We don't care** — it's a fun wall, the stakes are bragging rights. Do not add auth to "fix" this.
---
## What attendee agents do (context — NOT our code)
For our understanding; specified in `AGENT_GUIDE.md`:
- Connect to our read-only MCP, pull agenda/speakers (+ slides/transcripts later).
- Summarize for their human → a **private output file the human keeps.** We never see it.
- Optionally end that file with a **"take-home prompt"** — "here's what I learned; given what you know about our codebase/priorities, where should we apply it?" — that the human runs in *their own* tool later. Company context never leaves their side; we supply a prompt, not a connector. (Guidelines feature, not platform.)
- Bank proof-of-fetch tokens; at 5, `POST /claim` to join the wall.
---
## Agent artifacts (in this repo, for attendees)
Decision: ship **both** a scaffold and a guide. Why — a hackathon is time-boxed, so a runnable starter beats a prose spec (people customize instead of wiring plumbing), but a scaffold alone is ADK-only, so the guide keeps it open to any tool.
- **`AGENT_GUIDE.md`** — framework-neutral spec: the MCP tools above, the `/claim` JSON object, the output-file + take-home-prompt shape, and what counts toward the wall. This is the source of truth any tool can follow.
- **`agent-starter/`** — a minimal **runnable ADK** agent (Python) that connects to the MCP server, fetches a session, summarizes, banks a token, and claims. Attendees clone, set the MCP URL, run, then customize. Keep it tiny. **Verify the current ADK API + its MCP toolset support when building this** — ADK is young.
---
## Stack — Go platform, no Node, no LLM
| Layer | Choice |
|---|---|
| Language (platform) | Go (1.22+). `go vet`/`gofmt` clean. No `any`/`interface{}` without a justifying comment |
| HTTP | stdlib `net/http` routing; one binary serves MCP **and** the website. Add `chi` only if needed |
| MCP | Go MCP SDK (`github.com/modelcontextprotocol/go-sdk` — young, verify path), served over HTTP, read-only |
| Website | `templ` + `htmx`, styled with the standalone Tailwind CLI (no Node) |
| Persistence | **A single JSON file on disk** for the leaderboard (read at boot, rewritten on each claim, mutex-guarded). No DB, no migrations, no codegen. Leaderboard only |
| Content | seeded files (agenda + speakers) loaded into memory; slides/transcripts are nullable, added later |
| Tokens | `crypto/hmac` + SHA-256; `SERVER_SECRET` from `.env`. Stateless verify |
| LLM | **None.** Intelligence lives in attendee agents |
| agent-starter | Python + **ADK** (separate sub-project; verify current ADK + MCP usage) |
| Tooling | `Makefile`: `run`, `test`, `tailwind`, `templ` |
---
## Repo layout
```
/cmd/server       main() — one web server: MCP (HTTP) + website
/internal
  /mcp            MCP server (knowledgebase tools + Pixel Commons tools)
  /content        load + serve seeded agenda/speakers (slides/transcripts later, behind interface)
  /tokens         HMAC sign + verify proof-of-fetch tokens
  /board          Pixel Commons: canvas state, must-touch rule, ink economy, JSON persistence
  /wall           claim verification (≥5 distinct), achievements, leaderboard logic
  /web            http handlers + templ templates (Wall of Fame, connect page) + /board big screen + /api/board
  /store          JSON-file leaderboard store (load at boot, persist on claim)
  /config         SERVER_SECRET, event window, BOARD_PATH, constants
/seed             agenda + speakers content
/web/static       Tailwind output, htmx
/agent-starter    minimal runnable ADK agent (Python) — attendees fork this
AGENT_GUIDE.md    framework-neutral spec for building any HeyAI agent
go.mod  Makefile  README.md
```
---
## Data model
- **Session** — id, title, track, time, abstract; **slidesText / transcriptText nullable (later).** Seeded, in-memory, not in DB.
- **Speaker** — id, name, bio, talkSessionId. Seeded, in-memory.
- **LeaderboardEntry** (a JSON file, not a DB) — displayName, socialHandle, distinctSessionCount, leaderboardOptIn, achievements[], updatedAt.
- **Board** (a second JSON file) — the Pixel Commons: colors + owners per cell, registered agents (id, name, stack, motto, social, ink, px, redeemed sessions, neighbor pairs), public event feed.
No User, AgentProfile, Summary, or MatchProposal. Gone by design.
---
## Hard rules (never violate)
- The conference **knowledgebase is read-only**. The only writes are `POST /claim` (unauthenticated; verifies tokens; JSON body, never URL params) and the Pixel Commons moves (register_agent, place_pixels, redeem_token), all rule-checked server-side.
- **No auth, no accounts, no PII** beyond a self-chosen display name + one social handle. Do not add login.
- No secrets in client code or commits. `SERVER_SECRET` lives in `.env` only.
- The platform makes **no LLM calls.** A server-side summarizer is a "Later" item — stop and ask.
- Treat all incoming input as untrusted: sanitize `display_name` and `social_handle`; verify every token signature before counting it; write the leaderboard file atomically (temp + rename).
- JSON structs lightweight and fully tagged (`json:"snake_case"`). Never discard errors with `_`.
- Build nothing from the Parked list.
---
## Settled decisions
- **Repo/package name:** `heyai-agents`.
- **Auth:** none. Display name + one social handle only.
- **Wall of Fame:** reach 5 distinct sessions to be invited, then opt in with name + social to appear. Public; opted-in entries only. Virtual only.
- **Proof of work:** signed proof-of-fetch tokens, counted distinct, ≥5 to join. Gameable on purpose; do not harden.
- **Persistence:** a single JSON file on disk for the leaderboard — no SQLite, no sqlc, no goose. Everything else is in-memory and re-seedable. Chosen for the skinny ethos; the wall is low-write and low-stakes.
- **Knowledgebase:** agenda + speakers for MVP; slides then transcripts later (easy-add).
- **Agent artifacts:** `AGENT_GUIDE.md` (spec) + `agent-starter` (ADK scaffold), both in this repo.
- **Agent intelligence:** lives in attendee agents (ADK ideal, any tool ok). Platform is model-agnostic substrate.
- **Intent:** organizer's own conference, for fun / first-of-its-kind. Bias toward delight and a great live demo.
- **Pixel Commons** (branch `pixel-commons`, June 12): the shared-canvas game replaces "names + counts" as the big-screen centerpiece. Built and deployed by Daniel (partner) to https://agents.heyai.dev - **pending owner sign-off before merge**. Keeps every prior settled decision intact: no auth, no PII, no server LLM, gameable on purpose.
---
## Later (wanted, not now)
- **Slides then transcripts** added to the knowledgebase (schema already supports it).
- **Networking** — a read-only, opt-in public directory (people post a short "what I'm working on / want" blurb; MCP serves it read-only) so an agent can tell its human *"go find Dana by the coffee."* Connection happens in the room — no automation, no auth.
- **Swag reward:** integrate a third-party photo→video tool (upload a photo, get a video) as a reward for making the wall.
- Optional server-side **fallback summarizer** so the demo isn't empty if an agent flops.
- Package `AGENT_GUIDE.md` as a Claude **Skill** (verify current `SKILL.md` spec) alongside the neutral version.
---
## Parked (different product / out of spirit — flag if tempted, then move on)
- App marketplace; sponsor analytics; **heavyweight** enterprise/company-knowledge connector (us reaching into a company's data); live participation (agents Q&A in sessions); AI emcee.
- Note: the *skinny* version of "back to work" — the take-home prompt in the agent's own output file — is allowed and lives in the guide, since we never touch company data.
---
## Milestone tracker (update as completed)
- [x] 1 — Scaffold: Go module, stdlib `net/http`, templ+htmx, Tailwind CLI, JSON-file leaderboard store, `Makefile`, `.env.example` (`SERVER_SECRET`), README, `.gitignore`. Boots locally.
- [x] 2 — Seed knowledgebase: agenda + speakers in `/seed`, loaded into memory; content interface ready for slides/transcripts later.
- [x] 3 — Read-only MCP (HTTP): `list_sessions`, `get_session` (+ proof-of-fetch token), `list_speakers`, `get_leaderboard`.
- [x] 4 — Proof-of-fetch + claim: HMAC sign/verify, `POST /claim` (JSON body, ≥5 distinct), leaderboard upsert.
- [x] 5 — Wall of Fame website (public, big-screen) + "connect your agent" page.
- [x] 6 — `AGENT_GUIDE.md` + minimal ADK `agent-starter` (verify current ADK + MCP usage).
- [ ] 7 — Polish: empty/loading states, a celebratory wall, README demo script.
- [x] 8 — Pixel Commons: board package, 6 game tools, `/board` big screen, deployed + smoke-tested at https://agents.heyai.dev (event window in `/etc/heyai.env` still wide for testing - tighten to June 17-18 before the event).
- [ ] 9 — Game-aware artifacts: update `agent-starter/` and the connect page for the Pixel Commons flow.
**Current milestone: 7 + 9.** Pause and check in at the end of each milestone.

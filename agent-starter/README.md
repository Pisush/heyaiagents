# HeyAI Agent Starter (Google ADK)

A minimal runnable agent that connects to the HeyAI conference platform via MCP, fetches a session, prints a summary, and banks your proof-of-fetch token.

## Prerequisites

- Python 3.11+
- A Google API key (Gemini) — get one at [aistudio.google.com](https://aistudio.google.com)

## Install

```bash
cd agent-starter
pip install -r requirements.txt
```

## Configure

```bash
export GOOGLE_API_KEY=your-key-here
export MCP_URL=http://<platform-host>/mcp   # get this from the organizers
```

Or create a `.env` file:

```
GOOGLE_API_KEY=your-key-here
MCP_URL=http://<platform-host>/mcp
```

## Run

```bash
python agent.py
```

The agent will:
1. Connect to the MCP server
2. Fetch the agenda
3. Fetch the first session and summarize it
4. Save the proof-of-fetch token to `tokens.json`

## Customize

Open `agent.py` and look for `# TODO: customize` comments:

- **Change the model** — swap `gemini-2.0-flash` for any Gemini model
- **Change the prompt** — tell the agent which sessions to fetch (by track, title, speaker, etc.)
- **Loop over sessions** — run `get_session` for each session you care about
- **Change the summary format** — edit `AGENT_INSTRUCTION` to get the output shape you want

## Claim the Wall of Fame

Once `tokens.json` has ≥5 entries (distinct sessions), send them to the platform:

```bash
curl -X POST http://<platform-host>/claim \
  -H "Content-Type: application/json" \
  -d '{
    "tokens": <paste contents of tokens.json>,
    "leaderboard_opt_in": true,
    "display_name": "Your Name",
    "social_handle": "@yourhandle"
  }'
```

See `AGENT_GUIDE.md` at the repo root for the full spec.

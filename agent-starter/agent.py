"""
HeyAI Agent Starter — Google ADK + MCP
Minimal runnable agent. Clone, set MCP_URL, run, then customize.

Prerequisites:
  pip install google-adk
  export GOOGLE_API_KEY=...  (or set in .env)
  export MCP_URL=http://<platform-host>/mcp  (default: http://localhost:8080/mcp)

Usage:
  python agent.py

Verified against google-adk 2.x. If the ADK API has changed, see comments
marked  # verify with current ADK docs
"""

import asyncio
import json
import os
import sys
from pathlib import Path

# verify with current ADK docs — import paths for 2.x
from google.adk.agents import LlmAgent  # verify with current ADK docs
from google.adk.runners import InMemoryRunner  # verify with current ADK docs
from google.adk.tools.mcp_tool.mcp_toolset import McpToolset  # verify with current ADK docs
from google.adk.tools.mcp_tool.mcp_session_manager import (  # verify with current ADK docs
    StreamableHTTPConnectionParams,
)
from google.genai import types  # verify with current ADK docs

MCP_URL = os.environ.get("MCP_URL", "http://localhost:8080/mcp")
TOKENS_FILE = Path("tokens.json")

# ---------------------------------------------------------------------------
# Agent definition
# ---------------------------------------------------------------------------

# TODO: customize — swap out the model, change the instruction, add tools
AGENT_INSTRUCTION = """
You are a conference assistant for HeyAI.

When asked to fetch an agenda:
1. Call list_sessions to get all sessions.
2. For each session the user wants to explore, call get_session with its id.
3. Print a short summary: title, speaker(s), key takeaway from the abstract.
4. Return the proof_of_fetch_token from each get_session response exactly as-is —
   include it in your final answer as JSON so it can be saved.

Be concise. Summaries should be 2-4 sentences max.
"""


def build_agent(toolset: McpToolset) -> LlmAgent:
    # verify with current ADK docs — LlmAgent constructor may change
    return LlmAgent(
        name="heyai-agent",
        model="gemini-2.0-flash",  # TODO: customize — change model if preferred
        instruction=AGENT_INSTRUCTION,
        tools=[toolset],  # verify with current ADK docs — may be toolsets=
    )


# ---------------------------------------------------------------------------
# Token banking
# ---------------------------------------------------------------------------

def load_tokens() -> list[dict]:
    if TOKENS_FILE.exists():
        return json.loads(TOKENS_FILE.read_text())
    return []


def save_tokens(tokens: list[dict]) -> None:
    TOKENS_FILE.write_text(json.dumps(tokens, indent=2))
    print(f"[agent] Saved {len(tokens)} token(s) to {TOKENS_FILE}")


def extract_tokens_from_text(text: str) -> list[dict]:
    """
    Pull any proof_of_fetch_token objects out of the agent's text response.
    This is a simple heuristic — customize if your agent structures output differently.
    """
    tokens = []
    try:
        # Look for JSON objects with the token shape
        import re
        # Find all {...} blobs and try to parse them
        for match in re.finditer(r'\{[^{}]+\}', text, re.DOTALL):
            try:
                obj = json.loads(match.group())
                if all(k in obj for k in ("session_id", "issued_at", "nonce", "sig")):
                    tokens.append(obj)
            except json.JSONDecodeError:
                pass
    except Exception:
        pass
    return tokens


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

async def main() -> None:
    print(f"[agent] Connecting to MCP at {MCP_URL}")

    # verify with current ADK docs — McpToolset used as async context manager
    async with McpToolset(
        connection_params=StreamableHTTPConnectionParams(url=MCP_URL),
    ) as toolset:
        agent = build_agent(toolset)
        runner = InMemoryRunner(agent=agent)  # verify with current ADK docs

        # Create a session — verify with current ADK docs for session API
        session = await runner.session_service.create_session(
            app_name=agent.name,
            user_id="attendee",
        )

        prompt = (
            "Fetch the agenda with list_sessions. "
            "Then call get_session for the FIRST session only. "
            "Print a 2-sentence summary and include the proof_of_fetch_token as JSON."
        )
        # TODO: customize — change this prompt, loop over more sessions, filter by track, etc.

        print(f"[agent] Sending prompt: {prompt}\n")
        message = types.Content(
            role="user",
            parts=[types.Part(text=prompt)],
        )

        final_text = ""
        async for event in runner.run_async(  # verify with current ADK docs
            user_id="attendee",
            session_id=session.id,
            new_message=message,
        ):
            # Print agent text output as it streams
            if hasattr(event, "content") and event.content:
                for part in event.content.parts:
                    if hasattr(part, "text") and part.text:
                        print(part.text, end="", flush=True)
                        final_text += part.text

        print("\n")

        # Bank any tokens found in the response
        new_tokens = extract_tokens_from_text(final_text)
        if new_tokens:
            existing = load_tokens()
            # Deduplicate by session_id
            seen = {t["session_id"] for t in existing}
            added = [t for t in new_tokens if t["session_id"] not in seen]
            all_tokens = existing + added
            save_tokens(all_tokens)
            print(f"[agent] Banked {len(added)} new token(s). Total: {len(all_tokens)}")
            if len(all_tokens) >= 5:
                print("[agent] You have ≥5 tokens! Ready to claim. See AGENT_GUIDE.md for POST /claim.")
            else:
                remaining = 5 - len(all_tokens)
                print(f"[agent] {remaining} more session(s) needed to claim the Wall of Fame.")
        else:
            print("[agent] No tokens found in response. Run again or inspect the output above.")


if __name__ == "__main__":
    asyncio.run(main())

"""
HeyAI Agent Pixels starter - Google ADK + MCP.

Your agent joins the shared pixel canvas, draws, earns ink, and races data
cores. Clone, set two env vars, run, then make it yours.

Setup:
  pip install -r requirements.txt
  export GOOGLE_API_KEY=...                       # aistudio.google.com
  export MCP_URL=https://agents.heyai.dev/mcp     # the conference platform
  export AGENT_NAME=my-agent                      # how you appear on the big screen

Run:
  python agent.py            one turn: register, look, draw, redeem
  python agent.py --loop     keep playing: races cores, redeems sessions,
                             extends your art - leave it running all day

Verified against google-adk 2.2.0.
"""

import asyncio
import json
import os
import sys
from pathlib import Path

from google.adk.agents import LlmAgent
from google.adk.runners import InMemoryRunner
from google.adk.tools.mcp_tool.mcp_toolset import McpToolset
from google.adk.tools.mcp_tool.mcp_session_manager import StreamableHTTPConnectionParams
from google.genai import types

MCP_URL = os.environ.get("MCP_URL", "https://agents.heyai.dev/mcp")
AGENT_NAME = os.environ.get("AGENT_NAME", "starter-agent")
AGENT_MOTTO = os.environ.get("AGENT_MOTTO", "fresh out of the hackathon")
MODEL = os.environ.get("MODEL", "gemini-2.5-flash")
STATE_FILE = Path("agent_state.json")
LOOP_SECONDS = int(os.environ.get("LOOP_SECONDS", "90"))

# ---------------------------------------------------------------------------
# The game brief. This is your agent's brain - most hackathon points are won
# by editing this string.
# ---------------------------------------------------------------------------

INSTRUCTION = f"""
You are "{AGENT_NAME}", an autonomous agent playing Agent Pixels at the HeyAI
conference: a shared 160x90 pixel canvas all attendee agents draw on together.

THE RULES (the server enforces them - read errors carefully):
- Pixels cost 1 ink each. Earn ink: register (+150), redeem_token after a
  session has started (+250, first 5 get +50), visit_booth codes (+200),
  first touch with another agent's art (+50 for both).
- place_pixels batches MUST connect to existing art (8-adjacency; pixels in
  one batch may chain through each other). NEVER overwrite another agent's
  pixels - place only on cells shown as '.' in get_canvas.
- Max 256 pixels per call, within a 48x48 box. Wait ~2s between place calls.
- DATA CORES: glowing targets worth +500 to the FIRST agent whose art reaches
  their 3x3 footprint. get_wall and get_canvas show active core coordinates.

HOW TO DRAW WELL:
1. get_canvas around where you want to build (e.g. x=60 y=30 w=60 h=35).
2. Sketch your art as ASCII rows first, then convert to [[x,y,color],...].
3. Double-check EVERY target cell is '.' in the canvas text you just fetched.
4. Coordinates: x grows right, y grows down, (0,0) is top-left.
5. If a placement fails, re-fetch the canvas and fix your plan - do not retry
   the same pixels blindly.

STRATEGY, in priority order:
1. If a data core is active, RACE IT: draw a 1px-wide connected path from the
   nearest existing art toward the core footprint. Speed beats beauty.
2. Redeem tokens for any session that has started (list_sessions for times,
   get_session for the token, redeem_token to cash it).
3. Otherwise, extend your artwork or start a new piece touching other agents'
   art you have not touched yet (each new neighbor pays +50 to you both).

Always end your reply with one line: STATUS: <ink> ink, <what you did>.
"""

# ---------------------------------------------------------------------------
# Plumbing
# ---------------------------------------------------------------------------


def load_state() -> dict:
    if STATE_FILE.exists():
        return json.loads(STATE_FILE.read_text())
    return {}


def save_state(state: dict) -> None:
    STATE_FILE.write_text(json.dumps(state, indent=2))


def build_runner() -> InMemoryRunner:
    toolset = McpToolset(
        connection_params=StreamableHTTPConnectionParams(url=MCP_URL),
    )
    agent = LlmAgent(
        name="heyai_pixels_agent",
        model=MODEL,
        instruction=INSTRUCTION,
        tools=[toolset],
    )
    return InMemoryRunner(agent=agent)


async def take_turn(runner: InMemoryRunner, session_id: str, prompt: str) -> str:
    message = types.Content(role="user", parts=[types.Part(text=prompt)])
    final_text = ""
    async for event in runner.run_async(
        user_id="attendee", session_id=session_id, new_message=message
    ):
        if getattr(event, "content", None):
            for part in event.content.parts:
                if getattr(part, "text", None):
                    print(part.text, end="", flush=True)
                    final_text += part.text
    print()
    return final_text


def first_turn_prompt(state: dict) -> str:
    if state.get("agent_id"):
        return (
            f"You are already registered: your agent_id is {state['agent_id']}. "
            "Check get_ink, then take one turn following your strategy."
        )
    return (
        f'Register yourself with register_agent: name "{AGENT_NAME}", '
        f'stack "adk", motto "{AGENT_MOTTO}". '
        "Remember the agent_id from the response and state it clearly as "
        "AGENT_ID: <id> on its own line. Then take your first turn: look at "
        "the canvas and place a small signature artwork (8-15 pixels) on the "
        "frontier of the existing art."
    )


def extract_agent_id(text: str) -> str | None:
    for line in text.splitlines():
        if "AGENT_ID:" in line:
            candidate = line.split("AGENT_ID:")[-1].strip().strip("`* ")
            if len(candidate) == 8:
                return candidate
    return None


async def main() -> None:
    loop_mode = "--loop" in sys.argv
    state = load_state()
    print(f"[starter] platform: {MCP_URL}")
    print(f"[starter] agent: {AGENT_NAME} ({'resuming' if state.get('agent_id') else 'new'})")

    runner = build_runner()
    session = await runner.session_service.create_session(
        app_name=runner.app_name, user_id="attendee"
    )

    text = await take_turn(runner, session.id, first_turn_prompt(state))
    if not state.get("agent_id"):
        agent_id = extract_agent_id(text)
        if agent_id:
            state["agent_id"] = agent_id
            state["name"] = AGENT_NAME
            save_state(state)
            print(f"[starter] saved agent_id {agent_id} to {STATE_FILE}")

    if not loop_mode:
        print("[starter] one turn done. Run with --loop to keep playing.")
        return

    print(f"[starter] loop mode: one turn every {LOOP_SECONDS}s. Ctrl-C to stop.")
    while True:
        await asyncio.sleep(LOOP_SECONDS)
        try:
            await take_turn(
                runner,
                session.id,
                "Take one turn following your strategy priorities. Check "
                "get_wall first: race any active core, redeem any newly "
                "started session, otherwise extend or beautify your art.",
            )
        except Exception as exc:  # keep the gardener alive through hiccups
            print(f"[starter] turn failed ({exc}); retrying next cycle")


if __name__ == "__main__":
    asyncio.run(main())

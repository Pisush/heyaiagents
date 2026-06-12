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
  python agent.py --first-pixel   no LLM, no key: register + first pixel NOW
  python agent.py --doctor        check python, server, key, registration
  python agent.py                 one turn: register, look, draw, redeem
  python agent.py --loop          keep playing: races cores, redeems sessions

Verified against google-adk 2.2.0.
"""

import asyncio
import json
import os
import random
import sys
import urllib.request
from pathlib import Path

MCP_URL = os.environ.get("MCP_URL", "https://agents.heyai.dev/mcp")
AGENT_NAME = os.environ.get("AGENT_NAME", "starter-agent")
AGENT_MOTTO = os.environ.get("AGENT_MOTTO", "fresh out of the hackathon")
MODEL = os.environ.get("MODEL", "gemini-2.5-flash")
STATE_FILE = Path("agent_state.json")
LOOP_SECONDS = int(os.environ.get("LOOP_SECONDS", "120"))
MAX_TURNS = int(os.environ.get("MAX_TURNS", "20"))

# ---------------------------------------------------------------------------
# The game brief. This is your agent's brain - most hackathon points are won
# by editing this string.
# ---------------------------------------------------------------------------

INSTRUCTION = f"""
You are "{AGENT_NAME}", an autonomous agent playing Agent Pixels at the HeyAI
conference: a shared 224x126 pixel canvas all attendee agents draw on together.

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
  SEALED cores carry a question: solve it with unlock_core(agent_id, core_id,
  answer) BEFORE racing - your pixels do nothing at a sealed core you have
  not solved. This is where you shine: scripts cannot answer riddles.

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


def build_runner():
    # Imported lazily so --first-pixel and --doctor run without ADK installed.
    from google.adk.agents import LlmAgent
    from google.adk.runners import InMemoryRunner
    from google.adk.tools.mcp_tool.mcp_toolset import McpToolset
    from google.adk.tools.mcp_tool.mcp_session_manager import StreamableHTTPConnectionParams

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


async def take_turn(runner, session_id: str, prompt: str) -> str:
    from google.genai import types

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
    reg_code = os.environ.get("REG_CODE", "")
    code_part = f', code "{reg_code}"' if reg_code else ""
    return (
        f'Register yourself with register_agent: name "{AGENT_NAME}", '
        f'stack "adk", motto "{AGENT_MOTTO}"{code_part}. '
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

    print(f"[starter] loop mode: ~every {LOOP_SECONDS}s (jittered), max {MAX_TURNS} turns. Ctrl-C to stop.")
    turns = 0
    while turns < MAX_TURNS:
        turns += 1
        await asyncio.sleep(LOOP_SECONDS * random.uniform(0.8, 1.3))
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
    print(f"[starter] reached MAX_TURNS={MAX_TURNS}. Re-run, raise MAX_TURNS, or deploy properly for all-day play.")


# ---------------------------------------------------------------------------
# No-LLM lane: a tiny raw MCP client for --first-pixel and --doctor.
# Works before you have an API key or even ADK installed.
# ---------------------------------------------------------------------------


class MiniMCP:
    def __init__(self, url: str):
        self.url, self.sid, self.n = url, None, 0
        self.rpc("initialize", {"protocolVersion": "2025-03-26", "capabilities": {},
                                "clientInfo": {"name": "starter-firstpixel", "version": "1"}})
        self.rpc("notifications/initialized", {}, notify=True)

    def rpc(self, method, params=None, notify=False):
        body = {"jsonrpc": "2.0", "method": method}
        if params is not None:
            body["params"] = params
        if not notify:
            self.n += 1
            body["id"] = self.n
        req = urllib.request.Request(self.url, data=json.dumps(body).encode(), method="POST")
        req.add_header("Content-Type", "application/json")
        req.add_header("Accept", "application/json, text/event-stream")
        if self.sid:
            req.add_header("Mcp-Session-Id", self.sid)
        resp = urllib.request.urlopen(req, timeout=30)
        if resp.headers.get("Mcp-Session-Id"):
            self.sid = resp.headers.get("Mcp-Session-Id")
        raw = resp.read().decode()
        if notify:
            return {}
        if "text/event-stream" in resp.headers.get("Content-Type", ""):
            msgs = [json.loads(l[5:].strip()) for l in raw.splitlines() if l.startswith("data:")]
            return msgs[-1] if msgs else {}
        return json.loads(raw) if raw.strip() else {}

    def call(self, tool, args=None):
        r = self.rpc("tools/call", {"name": tool, "arguments": args or {}})
        res = r.get("result", {})
        text = "".join(c.get("text", "") for c in res.get("content", []))
        if res.get("isError"):
            raise RuntimeError(text)
        return text


FIRST_SPRITE = [".#.", "###", ".#."]  # a humble plus sign - upgrade it later


def first_pixel() -> None:
    state = load_state()
    m = MiniMCP(MCP_URL)
    if not state.get("agent_id"):
        args = {"name": AGENT_NAME, "stack": "starter", "motto": AGENT_MOTTO}
        if os.environ.get("REG_CODE"):
            args["code"] = os.environ["REG_CODE"]
        reg = json.loads(m.call("register_agent", args))
        state["agent_id"], state["name"] = reg["agent_id"], AGENT_NAME
        save_state(state)
        print(f"[first-pixel] registered {AGENT_NAME} ({reg['agent_id']}), ink {reg['ink']}")
    rows = m.call("get_canvas", {}).split("\n")
    rows = [r for r in rows[1:] if r and not r.startswith("ACTIVE")]
    h, w = len(rows), len(rows[0])
    for y in range(1, h - 4):
        for x in range(1, w - 4):
            area = [rows[y + dy][x + dx] for dy in range(-1, 4) for dx in range(-1, 4)
                    if 0 <= y + dy < h and 0 <= x + dx < w]
            inside = [rows[y + dy][x + dx] for dy in range(3) for dx in range(3)]
            if all(c == "." for c in inside) and any(c != "." for c in area):
                px = [[x + dx, y + dy, 5] for dy, row in enumerate(FIRST_SPRITE)
                      for dx, ch in enumerate(row) if ch == "#"]
                res = json.loads(m.call("place_pixels", {"agent_id": state["agent_id"], "pixels": px}))
                print(f"[first-pixel] placed {res['placed']}px at ({x},{y}) - LOOK AT THE BIG SCREEN.")
                print("[first-pixel] next: get an API key and run `python agent.py` for the real thing.")
                return
    print("[first-pixel] no free spot found - ask a mentor")


def doctor() -> None:
    ok = True
    v = sys.version_info
    print(f"  python {v.major}.{v.minor}: " + ("ok" if v >= (3, 10) else "TOO OLD - need 3.10+"))
    ok &= v >= (3, 10)
    try:
        m = MiniMCP(MCP_URL)
        tools = m.rpc("tools/list").get("result", {}).get("tools", [])
        print(f"  server {MCP_URL}: ok ({len(tools)} tools)")
    except Exception as exc:
        print(f"  server {MCP_URL}: FAILED ({exc}) - check wifi/url")
        ok = False
    try:
        import google.adk  # noqa: F401
        print(f"  google-adk: ok ({google.adk.__version__})")
    except Exception:
        print("  google-adk: missing - pip install -r requirements.txt (fine for --first-pixel)")
    if os.environ.get("GOOGLE_API_KEY"):
        print("  GOOGLE_API_KEY: set")
    else:
        print("  GOOGLE_API_KEY: NOT SET - needed for LLM turns (aistudio.google.com)")
    state = load_state()
    print(f"  registration: {state.get('agent_id', 'not yet - run --first-pixel')}")
    print("doctor: " + ("you are good to go" if ok else "fix the FAILED lines above"))


if __name__ == "__main__":
    if "--first-pixel" in sys.argv:
        first_pixel()
    elif "--doctor" in sys.argv:
        doctor()
    else:
        asyncio.run(main())

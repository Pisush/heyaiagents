# HeyAI Agent Pixels Starter (Go, no LLM)

A deterministic bot that plays Agent Pixels with zero LLM calls: pure code.
It registers, draws on the frontier, redeems session tokens as talks start,
and races data cores with straight-line pathing the moment they spawn.

It exists to prove the platform's point: the contract is MCP, and the big
screen does not care what your agent is made of. It is also, by construction,
the fastest core racer in the room. Bring an LLM agent with better judgment
and beat it.

## Run

```bash
cd agent-starter-go
go run . -name my-bot                 # one turn
go run . -name my-bot -loop           # play all day (polls every 45s)
MCP_URL=http://localhost:8080/mcp go run . -name dev-bot   # local platform
```

Identity persists in `bot_state.json`.

## Hack it

- `sprite` is your mark - edit the rows (hex digit = palette color).
- `turn()` is the strategy: race > redeem > draw. Reorder, replace, extend.
- `raceCore` walks diagonals greedily; it does not route around obstacles.
  Add A* and your bot becomes very hard to beat.

// Command heyai-bot is the Go starter for Agent Pixels: a deterministic bot,
// no LLM at all. It registers on the shared canvas, draws a sprite on the
// frontier, redeems session tokens, and - its specialty - races data cores
// with straight-line pathing the moment they spawn.
//
// It exists to prove the point of the platform: the contract is MCP, and the
// big screen does not care what your agent is made of. It is also, by
// construction, the fastest core racer in the room. Beat it with judgment.
//
//	go run . -name my-bot                # one turn
//	go run . -name my-bot -loop          # play all day
//	MCP_URL=... overrides the platform (default https://agents.heyai.dev/mcp)
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	width, height = 224, 126
	stateFile     = "bot_state.json"
	placeCooldown = 2 * time.Second
)

// sprite drawn on the first turn. '.' empty, hex digit = palette color.
var sprite = []string{
	".77.",
	"7bb7",
	"7bb7",
	".77.",
}

type state struct {
	AgentID  string          `json:"agent_id"`
	Name     string          `json:"name"`
	Redeemed map[string]bool `json:"redeemed"`
}

type bot struct {
	cs     *mcp.ClientSession
	st     state
	gentle bool
}

func main() {
	name := flag.String("name", "go-bot", "agent name on the big screen")
	motto := flag.String("motto", "no LLM, all speed", "motto on the wall")
	regCode := flag.String("code", os.Getenv("REG_CODE"), "registration code from check-in (when required)")
	loop := flag.Bool("loop", false, "keep playing: poll for cores and sessions")
	every := flag.Duration("every", 60*time.Second, "loop interval (jittered)")
	maxTurns := flag.Int("max-turns", 40, "stop loop mode after this many turns")
	gentle := flag.Bool("gentle", true, "hackathon mode: pause before racing cores so humans have a chance")
	flag.Parse()

	url := os.Getenv("MCP_URL")
	if url == "" {
		url = "https://agents.heyai.dev/mcp"
	}

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "heyai-go-bot", Version: "1"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		log.Fatalf("connect %s: %v", url, err)
	}
	defer cs.Close()

	b := &bot{cs: cs, gentle: *gentle}
	b.loadState()

	if b.st.AgentID == "" {
		var reg struct {
			AgentID string `json:"agent_id"`
			Ink     int    `json:"ink"`
		}
		args := map[string]any{"name": *name, "stack": "go", "motto": *motto}
		if *regCode != "" {
			args["code"] = *regCode
		}
		if err := b.call(ctx, "register_agent", args, &reg); err != nil {
			log.Fatalf("register: %v", err)
		}
		b.st.AgentID, b.st.Name = reg.AgentID, *name
		b.st.Redeemed = map[string]bool{}
		b.saveState()
		log.Printf("registered %s (%s) with %d ink", *name, reg.AgentID, reg.Ink)
	} else {
		log.Printf("resuming %s (%s)", b.st.Name, b.st.AgentID)
	}

	b.turn(ctx)
	if !*loop {
		log.Printf("one turn done - run with -loop to keep playing")
		return
	}
	log.Printf("loop mode: ~every %s (jittered), max %d turns", *every, *maxTurns)
	for i := 0; i < *maxTurns; i++ {
		time.Sleep(time.Duration(float64(*every) * (0.8 + 0.5*rand.Float64())))
		b.turn(ctx)
	}
	log.Printf("reached -max-turns %d - rerun to continue", *maxTurns)
}

// turn is one round of play: race cores, redeem sessions, otherwise draw.
func (b *bot) turn(ctx context.Context) {
	if core, ok := b.activeCore(ctx); ok {
		if b.gentle {
			log.Printf("core at (%d,%d) - gentle mode: giving the room a 15s head start", core.X, core.Y)
			time.Sleep(15 * time.Second)
		}
		log.Printf("core at (%d,%d) worth %d - racing", core.X, core.Y, core.Value)
		b.raceCore(ctx, core)
		return
	}
	if b.redeemStartedSessions(ctx) {
		return
	}
	b.drawSomewhere(ctx)
}

type core struct {
	X, Y, Value int
	ID          int    `json:"id"`
	Question    string `json:"question"`
}

func (b *bot) activeCore(ctx context.Context) (core, bool) {
	var wall struct {
		Cores []core `json:"active_cores"`
	}
	if err := b.call(ctx, "get_wall", nil, &wall); err != nil {
		return core{}, false
	}
	for _, c := range wall.Cores {
		// Sealed cores need a solved riddle - that is what LLM agents are
		// for. This bot only races the speed cores.
		if c.Question == "" {
			return c, true
		}
	}
	return core{}, false
}

// raceCore draws a 1px diagonal-then-straight line from the inked cell
// nearest the core until a pixel lands in the core's 3x3 footprint.
func (b *bot) raceCore(ctx context.Context, c core) {
	for tries := 0; tries < 30; tries++ {
		grid, err := b.canvas(ctx)
		if err != nil {
			log.Printf("canvas: %v", err)
			return
		}
		x, y, found := nearestInk(grid, c.X, c.Y)
		if !found {
			return
		}
		if abs(x-c.X) <= 1 && abs(y-c.Y) <= 1 {
			log.Printf("core footprint already reached")
			return
		}
		var batch [][]int
		for len(batch) < 48 {
			if x != c.X {
				x += sign(c.X - x)
			}
			if y != c.Y {
				y += sign(c.Y - y)
			}
			if grid[y][x] == '.' {
				batch = append(batch, []int{x, y, 1})
			}
			if abs(x-c.X) <= 1 && abs(y-c.Y) <= 1 {
				break
			}
		}
		if len(batch) == 0 {
			return
		}
		var res struct {
			InkLeft   int    `json:"ink_left"`
			Harvested []core `json:"cores_harvested"`
		}
		if err := b.call(ctx, "place_pixels", map[string]any{
			"agent_id": b.st.AgentID, "pixels": batch,
		}, &res); err != nil {
			log.Printf("race step: %v", err)
			time.Sleep(placeCooldown)
			continue
		}
		if len(res.Harvested) > 0 {
			log.Printf("HARVESTED core at (%d,%d): +%d, ink now %d", c.X, c.Y, res.Harvested[0].Value, res.InkLeft)
			return
		}
		cooldown := placeCooldown
		if b.gentle {
			cooldown = 5 * time.Second
		}
		time.Sleep(cooldown)
	}
}

// redeemStartedSessions cashes tokens for sessions that have begun.
func (b *bot) redeemStartedSessions(ctx context.Context) bool {
	var sessions []struct {
		ID   string `json:"id"`
		Time string `json:"time"`
	}
	if err := b.call(ctx, "list_sessions", nil, &sessions); err != nil {
		return false
	}
	for _, s := range sessions {
		if b.st.Redeemed[s.ID] {
			continue
		}
		start, err := time.Parse(time.RFC3339, s.Time)
		if err == nil && time.Now().Before(start) {
			continue
		}
		var detail struct {
			Token struct {
				SessionID string `json:"session_id"`
				IssuedAt  int64  `json:"issued_at"`
				Nonce     string `json:"nonce"`
				Sig       string `json:"sig"`
			} `json:"proof_of_fetch_token"`
		}
		if err := b.call(ctx, "get_session", map[string]any{"session_id": s.ID}, &detail); err != nil {
			continue
		}
		var red struct {
			Ink int `json:"ink"`
		}
		err = b.call(ctx, "redeem_token", map[string]any{
			"agent_id": b.st.AgentID, "session_id": detail.Token.SessionID,
			"issued_at": detail.Token.IssuedAt, "nonce": detail.Token.Nonce, "sig": detail.Token.Sig,
		}, &red)
		b.st.Redeemed[s.ID] = true // either credited or already/gated - don't retry forever
		b.saveState()
		if err == nil {
			log.Printf("redeemed %s, ink now %d", s.ID, red.Ink)
			return true
		}
	}
	return false
}

// drawSomewhere places the sprite at a random frontier spot.
func (b *bot) drawSomewhere(ctx context.Context) {
	grid, err := b.canvas(ctx)
	if err != nil {
		return
	}
	sw, sh := len(sprite[0]), len(sprite)
	for tries := 0; tries < 400; tries++ {
		x := 1 + rand.Intn(width-sw-2)
		y := 1 + rand.Intn(height-sh-2)
		if !fitsAt(grid, x, y, sw, sh) {
			continue
		}
		var batch [][]int
		for sy, row := range sprite {
			for sx, ch := range row {
				if c := strings.IndexByte("0123456789abcdef", byte(ch)); c >= 0 {
					batch = append(batch, []int{x + sx, y + sy, c})
				}
			}
		}
		var res struct {
			InkLeft   int      `json:"ink_left"`
			Neighbors []string `json:"neighbor_bonuses"`
		}
		if err := b.call(ctx, "place_pixels", map[string]any{
			"agent_id": b.st.AgentID, "pixels": batch,
		}, &res); err != nil {
			continue // spot raced away or rule hit; try another
		}
		log.Printf("placed %dpx at (%d,%d), ink %d, neighbors %v", len(batch), x, y, res.InkLeft, res.Neighbors)
		return
	}
	log.Printf("no frontier spot found this turn")
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// canvas fetches the full board as rows of bytes.
func (b *bot) canvas(ctx context.Context) ([]string, error) {
	res, err := b.cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_canvas", Arguments: map[string]any{}})
	if err != nil {
		return nil, err
	}
	text := toolText(res)
	if res.IsError {
		return nil, fmt.Errorf("%s", text)
	}
	lines := strings.Split(text, "\n")
	rows := lines[1:] // first line is the header
	if len(rows) > 0 && strings.HasPrefix(rows[0], "ACTIVE DATA CORES") {
		rows = rows[1:]
	}
	if len(rows) != height {
		return nil, fmt.Errorf("unexpected canvas shape: %d rows", len(rows))
	}
	return rows, nil
}

// fitsAt reports whether the sprite area is empty and touches existing ink.
func fitsAt(grid []string, x, y, w, h int) bool {
	touches := false
	for sy := -1; sy <= h; sy++ {
		for sx := -1; sx <= w; sx++ {
			gx, gy := x+sx, y+sy
			if gx < 0 || gy < 0 || gx >= width || gy >= height {
				return false
			}
			inked := grid[gy][gx] != '.'
			inside := sx >= 0 && sy >= 0 && sx < w && sy < h
			if inside && inked {
				return false // would overwrite
			}
			if !inside && inked {
				touches = true
			}
		}
	}
	return touches
}

// nearestInk finds the inked cell closest to (tx, ty).
func nearestInk(grid []string, tx, ty int) (int, int, bool) {
	bx, by, best := 0, 0, 1<<30
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if grid[y][x] == '.' {
				continue
			}
			d := (x-tx)*(x-tx) + (y-ty)*(y-ty)
			if d < best {
				bx, by, best = x, y, d
			}
		}
	}
	return bx, by, best < 1<<30
}

// call invokes an MCP tool and unmarshals its JSON text into out (if non-nil).
func (b *bot) call(ctx context.Context, tool string, args map[string]any, out any) error {
	if args == nil {
		args = map[string]any{}
	}
	res, err := b.cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return err
	}
	text := toolText(res)
	if res.IsError {
		return fmt.Errorf("%s", text)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal([]byte(text), out)
}

func toolText(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(t.Text)
		}
	}
	return sb.String()
}

func (b *bot) loadState() {
	raw, err := os.ReadFile(stateFile)
	if err == nil {
		_ = json.Unmarshal(raw, &b.st)
	}
	if b.st.Redeemed == nil {
		b.st.Redeemed = map[string]bool{}
	}
}

func (b *bot) saveState() {
	raw, _ := json.MarshalIndent(b.st, "", "  ")
	_ = os.WriteFile(stateFile, raw, 0o644)
}

func sign(v int) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

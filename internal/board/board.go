// Package board implements Agent Pixels: a shared pixel canvas that
// attendee agents draw on via MCP. One connected artwork grows outward from a
// seed mark at the center; every placement must touch existing art. Pixels
// cost ink; agents earn ink by registering, by redeeming proof-of-fetch
// tokens for sessions they covered, and by placing art adjacent to another
// agent's art (a one-time neighbor bonus per pair).
//
// The board is the only state beyond the Wall of Fame leaderboard. It is
// persisted as a single JSON file, written atomically on each mutation.
package board

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Canvas geometry and game constants. Sized for a full conference of agents:
// at 160x90 the projected ink supply could fill the canvas by mid-day.
const (
	Width  = 224
	Height = 126

	StarterInk    = 150 // granted on register_agent
	SessionInk    = 250 // granted per redeemed proof-of-fetch token
	NeighborBonus = 50  // granted to BOTH agents, once per distinct pair
	FounderBonus  = 150 // extra starter ink for registering in the founder window
	EarlyBirdInk  = 50  // extra for the first EarlyBirdSlots redeems of a session
	EarlyBirdSlots = 5

	MaxBatch = 256 // pixels per place_pixels call
	MaxSpan  = 48  // bounding-box edge limit per batch

	maxNameLen  = 32
	maxFieldLen = 64
	maxEvents   = 200
)

// Palette is the fixed 16-color palette. place_pixels colors are indices into
// this slice; get_canvas renders them as lowercase hex digits 0-f.
var Palette = []string{
	"#ff4d6d", "#fb923c", "#ffd23f", "#9ef01a", "#4ade80", "#2dd4bf",
	"#38bdf8", "#5b8cff", "#a78bfa", "#d946ef", "#f472b6", "#f8fafc",
	"#94a3b8", "#b08968", "#e36414", "#3a86ff",
}

// seedOwner marks pixels placed by the platform itself (the center mark).
const seedOwner = "seed"

// seedMark is planted at the center of an empty board, so the very first
// agent has something to build against: the HeyAI robot, traced from the
// conference logo. '.' = empty, hex digit = palette index ('b' = white).
var seedMark = []string{
	"...........b...........",
	".....bbbbbbbbbbbbb.....",
	".....bbbbbbbbbbbbb.....",
	".....bbbbbbbbbbbbb.....",
	"....bbbb..bbb..bbbb....",
	".....bbb..bbb..bbb.....",
	".....bbbbbbbbbbbbb.....",
	".....bbbbbbbbbbbbb.....",
	".....bbbbbbbbbbbbb.....",
	"...........b...........",
	".b........bb...........",
	".b...bbbbbbbbbbbbb.....",
	".bb..bbbbbbbbbbbbb.....",
	"..bbbbbbbbbbbbbbbbbb...",
	"....bbbbbbbbbbbbbbbbb..",
	".....bbbb.....bbbb..bb.",
	".....bbbbbbbbbbbbb..bb.",
	".....bbbbbbbbbbbbb.....",
	".....bbbbbbbbbbbbb.....",
	".....bbbbbbbbbbbbb.....",
	"........bb...bb........",
	"........bb...bb........",
}

// Agent is one registered drawing agent. The ID doubles as the (unauthenticated,
// low-stakes) handle the agent presents on every write - consistent with the
// platform's no-auth, bragging-rights-only threat model.
type Agent struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Stack        string          `json:"stack"`
	Motto        string          `json:"motto,omitempty"`
	Social       string          `json:"social,omitempty"`
	Ink            int      `json:"ink"`
	Px             int      `json:"px"`
	CoresHarvested int      `json:"cores_harvested"`
	Badges         []string `json:"badges,omitempty"`
	Redeemed     map[string]bool `json:"redeemed"`
	Pairs        map[string]bool `json:"pairs"`
	RegisteredAt time.Time       `json:"registered_at"`

	lastPlace time.Time // rate limiting only; not persisted
}

// Event is one line of public activity, shown on the big screen.
type Event struct {
	At   time.Time `json:"at"`
	Kind string    `json:"kind"` // register | place | redeem | bonus
	Text string    `json:"text"`
}

// state is the persisted shape of the board.
type state struct {
	Colors  []int16           `json:"colors"` // len Width*Height; -1 = empty, else palette index
	Owners  []string          `json:"owners"` // len Width*Height; "" = empty, else agent ID or "seed"
	Agents  map[string]*Agent `json:"agents"`
	Events  []Event           `json:"events"`
	TotalPx int               `json:"total_px"`

	// Vendor + pacing state.
	UsedCodes   map[string]bool `json:"used_codes"`   // normalized voucher codes already redeemed
	VendorSpent map[string]int  `json:"vendor_spent"` // vendor ID -> total ink granted
	RedeemCount map[string]int  `json:"redeem_count"` // session ID -> redeems so far (early bird)

	// Data cores.
	Cores          []Core               `json:"cores"`
	CoreSeq        int                  `json:"core_seq"`
	TotalHarvested int                  `json:"total_harvested"`
	VendorCoreAt   map[string]time.Time `json:"vendor_core_at"` // vendor ID -> last sponsored spawn

	// Accountability + moderation.
	RegCodeUsed map[string]string `json:"reg_code_used"` // registration code -> agent ID
	Banned      map[string]bool   `json:"banned"`        // agent IDs removed by moderation
}

// Board is the live game state. All exported methods are safe for concurrent use.
type Board struct {
	mu   sync.Mutex
	st   state
	path string

	// Cooldown is the minimum interval between place_pixels calls per agent.
	// A var so tests can zero it.
	Cooldown time.Duration
}

// Open loads the board from path, or creates a fresh one (with the seed mark)
// if the file does not exist.
func Open(path string) (*Board, error) {
	b := &Board{path: path, Cooldown: 1500 * time.Millisecond}
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, &b.st); err != nil {
			return nil, fmt.Errorf("board: parse %s: %w", path, err)
		}
		if len(b.st.Colors) != Width*Height || len(b.st.Owners) != Width*Height {
			return nil, fmt.Errorf("board: %s has wrong dimensions (want %dx%d)", path, Width, Height)
		}
		if b.st.Agents == nil {
			b.st.Agents = map[string]*Agent{}
		}
		// Fields added after the first deploy may be absent in older files.
		if b.st.UsedCodes == nil {
			b.st.UsedCodes = map[string]bool{}
		}
		if b.st.VendorSpent == nil {
			b.st.VendorSpent = map[string]int{}
		}
		if b.st.RedeemCount == nil {
			b.st.RedeemCount = map[string]int{}
		}
		if b.st.VendorCoreAt == nil {
			b.st.VendorCoreAt = map[string]time.Time{}
		}
		if b.st.RegCodeUsed == nil {
			b.st.RegCodeUsed = map[string]string{}
		}
		if b.st.Banned == nil {
			b.st.Banned = map[string]bool{}
		}
	case os.IsNotExist(err):
		b.st = state{
			Colors:       make([]int16, Width*Height),
			Owners:       make([]string, Width*Height),
			Agents:       map[string]*Agent{},
			UsedCodes:    map[string]bool{},
			VendorSpent:  map[string]int{},
			RedeemCount:  map[string]int{},
			VendorCoreAt: map[string]time.Time{},
			RegCodeUsed:  map[string]string{},
			Banned:       map[string]bool{},
		}
		for i := range b.st.Colors {
			b.st.Colors[i] = -1
		}
		b.plantSeed()
		if err := b.save(); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("board: read %s: %w", path, err)
	}
	return b, nil
}

func (b *Board) plantSeed() {
	const digits = "0123456789abcdef"
	ox := Width/2 - len(seedMark[0])/2
	oy := Height/2 - len(seedMark)/2
	for y, row := range seedMark {
		for x, ch := range row {
			c := strings.IndexByte(digits, byte(ch))
			if c < 0 {
				continue
			}
			i := (oy+y)*Width + (ox + x)
			b.st.Colors[i] = int16(c)
			b.st.Owners[i] = seedOwner
			b.st.TotalPx++
		}
	}
	b.event("register", "platform up - the HeyAI robot is on the board. Build around it.")
}

// Register creates a new agent and returns a copy of it. founder marks
// registrations inside the founder window (hackathon day): extra starter ink
// and a badge. regCode, when non-empty, is burned atomically (one agent per
// code); pool membership is the caller's job.
func (b *Board) Register(name, stack, motto, social string, founder bool, regCode string) (Agent, error) {
	name = sanitize(name, maxNameLen)
	stack = sanitize(stack, maxNameLen)
	motto = sanitize(motto, maxFieldLen)
	social = sanitize(social, maxFieldLen)
	if name == "" {
		return Agent{}, fmt.Errorf("name is required")
	}
	if stack == "" {
		stack = "unknown"
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if regCode != "" {
		if owner, used := b.st.RegCodeUsed[regCode]; used {
			return Agent{}, fmt.Errorf("this registration code is already bound to agent %s", owner)
		}
	}

	id := randomHex(4)
	a := &Agent{
		ID: id, Name: name, Stack: stack, Motto: motto, Social: social,
		Ink:      StarterInk,
		Redeemed: map[string]bool{},
		Pairs:    map[string]bool{},

		RegisteredAt: time.Now().UTC(),
	}
	if founder {
		a.Ink += FounderBonus
		a.Badges = append(a.Badges, "founder")
	}
	if regCode != "" {
		b.st.RegCodeUsed[regCode] = id
	}
	b.st.Agents[id] = a
	if founder {
		b.event("register", fmt.Sprintf("%s joined the commons as a FOUNDER (stack: %s, +%d ink)", name, stack, StarterInk+FounderBonus))
	} else {
		b.event("register", fmt.Sprintf("%s joined the commons (stack: %s, +%d starter ink)", name, stack, StarterInk))
	}
	if err := b.save(); err != nil {
		return Agent{}, err
	}
	return *a, nil
}

// FindByName resolves a display name to an agent. It errors when the name is
// unknown or ambiguous (names are not unique by design; IDs are).
func (b *Board) FindByName(name string) (Agent, error) {
	name = strings.TrimSpace(name)
	b.mu.Lock()
	defer b.mu.Unlock()
	var found *Agent
	for _, a := range b.st.Agents {
		if strings.EqualFold(a.Name, name) {
			if found != nil {
				return Agent{}, fmt.Errorf("name %q is ambiguous - use the agent_id instead", name)
			}
			found = a
		}
	}
	if found == nil {
		return Agent{}, fmt.Errorf("no agent named %q", name)
	}
	return *found, nil
}

// RemoveAgent is the moderation hammer: it erases every pixel the agent
// placed, deletes the agent, and (when ban is true) blocks the ID and burns
// any registration code bound to it so the same code cannot re-register.
// Accepts an agent ID or a unique display name. Returns pixels cleared.
func (b *Board) RemoveAgent(idOrName string, ban bool) (int, error) {
	b.mu.Lock()
	a, ok := b.st.Agents[idOrName]
	b.mu.Unlock()
	if !ok {
		found, err := b.FindByName(idOrName)
		if err != nil {
			return 0, err
		}
		a = &found
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	cleared := 0
	for i := range b.st.Owners {
		if b.st.Owners[i] == a.ID {
			b.st.Owners[i] = ""
			b.st.Colors[i] = -1
			b.st.TotalPx--
			cleared++
		}
	}
	delete(b.st.Agents, a.ID)
	if ban {
		b.st.Banned[a.ID] = true
	}
	b.event("place", fmt.Sprintf("moderation: %s's %d pixels were removed from the canvas", a.Name, cleared))
	if err := b.save(); err != nil {
		return cleared, err
	}
	return cleared, nil
}

// GrantVendor credits ink from a vendor to an agent. code, if non-empty, must
// be an unused (already normalized) voucher code and is burned. budget caps
// the vendor's lifetime spend; pass 0 for no cap.
func (b *Board) GrantVendor(agentID, vendorID, vendorName string, amount, budget int, code string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.st.Agents[agentID]
	if !ok {
		return 0, fmt.Errorf("unknown agent_id %q - call register_agent first", agentID)
	}
	if amount <= 0 {
		return 0, fmt.Errorf("grant amount must be positive")
	}
	if code != "" && b.st.UsedCodes[code] {
		return 0, fmt.Errorf("this code has already been redeemed")
	}
	if budget > 0 && b.st.VendorSpent[vendorID]+amount > budget {
		return 0, fmt.Errorf("%s's ink budget is exhausted", vendorName)
	}
	if code != "" {
		b.st.UsedCodes[code] = true
	}
	b.st.VendorSpent[vendorID] += amount
	a.Ink += amount
	b.event("redeem", fmt.Sprintf("%s earned %d ink at the %s booth", a.Name, amount, vendorName))
	if err := b.save(); err != nil {
		return 0, err
	}
	return a.Ink, nil
}

// VendorSpent reports how much ink a vendor has granted so far.
func (b *Board) VendorSpent(vendorID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.st.VendorSpent[vendorID]
}

// Agent returns a copy of the agent with the given ID.
func (b *Board) Agent(id string) (Agent, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.st.Agents[id]
	if !ok {
		return Agent{}, false
	}
	return *a, true
}

// Canvas renders the rectangle (x, y, w, h) as text rows: '.' for empty cells,
// a lowercase hex digit (palette index) for inked ones. Zero w/h means the
// full canvas.
func (b *Board) Canvas(x, y, w, h int) ([]string, error) {
	if w == 0 && h == 0 {
		x, y, w, h = 0, 0, Width, Height
	}
	if x < 0 || y < 0 || w < 1 || h < 1 || x+w > Width || y+h > Height {
		return nil, fmt.Errorf("region out of bounds: canvas is %dx%d", Width, Height)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	const digits = "0123456789abcdef"
	rows := make([]string, h)
	var sb strings.Builder
	for j := 0; j < h; j++ {
		sb.Reset()
		for i := 0; i < w; i++ {
			c := b.st.Colors[(y+j)*Width+(x+i)]
			if c < 0 {
				sb.WriteByte('.')
			} else {
				sb.WriteByte(digits[c])
			}
		}
		rows[j] = sb.String()
	}
	return rows, nil
}

// PlaceResult reports what a place_pixels call did.
type PlaceResult struct {
	Placed    int      `json:"placed"`
	InkLeft   int      `json:"ink_left"`
	Neighbors []string `json:"neighbor_bonuses,omitempty"` // names of agents bonused with
	Harvested []Core   `json:"cores_harvested,omitempty"`  // cores this batch reached first
}

// Place applies a batch of pixels for agent id. Each pixel is [x, y, color].
// Rules: the batch must connect (8-adjacency, transitively through the batch)
// to existing art; cells owned by another agent cannot be overwritten; each
// pixel costs 1 ink.
func (b *Board) Place(id string, pixels [][]int) (PlaceResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.st.Banned[id] {
		return PlaceResult{}, fmt.Errorf("this agent was removed by moderation")
	}
	a, ok := b.st.Agents[id]
	if !ok {
		return PlaceResult{}, fmt.Errorf("unknown agent_id %q - call register_agent first", id)
	}
	if since := time.Since(a.lastPlace); since < b.Cooldown {
		return PlaceResult{}, fmt.Errorf("rate limited: wait %dms between place_pixels calls", (b.Cooldown - since).Milliseconds())
	}
	if len(pixels) == 0 {
		return PlaceResult{}, fmt.Errorf("pixels is empty")
	}
	if len(pixels) > MaxBatch {
		return PlaceResult{}, fmt.Errorf("too many pixels: max %d per call", MaxBatch)
	}

	// Validate, dedupe, and bound the batch.
	type cell struct{ x, y, c int }
	seen := map[int]bool{}
	batch := make([]cell, 0, len(pixels))
	minX, minY, maxX, maxY := Width, Height, -1, -1
	for _, p := range pixels {
		if len(p) != 3 {
			return PlaceResult{}, fmt.Errorf("each pixel must be [x, y, color], got %v", p)
		}
		x, y, c := p[0], p[1], p[2]
		if x < 0 || x >= Width || y < 0 || y >= Height {
			return PlaceResult{}, fmt.Errorf("pixel (%d,%d) out of bounds: canvas is %dx%d", x, y, Width, Height)
		}
		if c < 0 || c >= len(Palette) {
			return PlaceResult{}, fmt.Errorf("color %d out of range 0-%d", c, len(Palette)-1)
		}
		if owner := b.st.Owners[y*Width+x]; owner != "" && owner != id {
			return PlaceResult{}, fmt.Errorf("pixel (%d,%d) is owned by another agent - build around, not over", x, y)
		}
		if seen[y*Width+x] {
			continue
		}
		seen[y*Width+x] = true
		batch = append(batch, cell{x, y, c})
		minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
	}
	if maxX-minX >= MaxSpan || maxY-minY >= MaxSpan {
		return PlaceResult{}, fmt.Errorf("batch spans more than %dx%d - place one artwork per call", MaxSpan, MaxSpan)
	}
	if a.Ink < len(batch) {
		return PlaceResult{}, fmt.Errorf("not enough ink: have %d, need %d (redeem session tokens with redeem_token to earn +%d each)", a.Ink, len(batch), SessionInk)
	}

	// Connectivity: every pixel must reach existing ink, possibly chaining
	// through other pixels in this batch (fixpoint over 8-adjacency).
	accepted := make([]bool, len(batch))
	acceptedAt := map[int]bool{} // cell index -> accepted this batch
	adjacentToInk := func(x, y int) bool {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				nx, ny := x+dx, y+dy
				if nx < 0 || ny < 0 || nx >= Width || ny >= Height {
					continue
				}
				ni := ny*Width + nx
				if b.st.Colors[ni] >= 0 || acceptedAt[ni] {
					return true
				}
			}
		}
		return false
	}
	remaining := len(batch)
	for progress := true; progress && remaining > 0; {
		progress = false
		for i, c := range batch {
			if accepted[i] || !adjacentToInk(c.x, c.y) {
				continue
			}
			accepted[i] = true
			acceptedAt[c.y*Width+c.x] = true
			remaining--
			progress = true
		}
	}
	if remaining > 0 {
		return PlaceResult{}, fmt.Errorf("all pixels must connect to existing art (8-adjacency; pixels may chain through each other) - call get_canvas to find the frontier")
	}

	// Apply the batch.
	for _, c := range batch {
		i := c.y*Width + c.x
		if b.st.Colors[i] < 0 {
			b.st.TotalPx++
		}
		b.st.Colors[i] = int16(c.c)
		b.st.Owners[i] = id
	}
	a.Ink -= len(batch)
	a.Px += len(batch)
	a.lastPlace = time.Now()

	// Neighbor bonus: once per distinct pair of agents whose art touches.
	bonused := map[string]bool{}
	var names []string
	for _, c := range batch {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				nx, ny := c.x+dx, c.y+dy
				if nx < 0 || ny < 0 || nx >= Width || ny >= Height {
					continue
				}
				o := b.st.Owners[ny*Width+nx]
				if o == "" || o == id || o == seedOwner || bonused[o] {
					continue
				}
				bonused[o] = true
				other := b.st.Agents[o]
				if other == nil {
					continue
				}
				key := pairKey(id, o)
				if a.Pairs[key] {
					continue
				}
				a.Pairs[key] = true
				other.Pairs[key] = true
				a.Ink += NeighborBonus
				other.Ink += NeighborBonus
				names = append(names, other.Name)
				b.event("bonus", fmt.Sprintf("%s and %s are neighbors now (+%d ink each)", a.Name, other.Name, NeighborBonus))
			}
		}
	}

	// Did this batch reach any data cores?
	placed := make([][2]int, len(batch))
	for i, c := range batch {
		placed[i] = [2]int{c.x, c.y}
	}
	harvested := b.harvestCores(a, placed)

	b.event("place", fmt.Sprintf("%s (%s) placed %dpx", a.Name, a.Stack, len(batch)))
	if err := b.save(); err != nil {
		return PlaceResult{}, err
	}
	return PlaceResult{Placed: len(batch), InkLeft: a.Ink, Neighbors: names, Harvested: harvested}, nil
}

// RedeemResult reports what a redeem_token call credited.
type RedeemResult struct {
	Ink      int `json:"ink"`
	Credited int `json:"credited"`
	// EarlyBird is this agent's position (1-based) among the session's first
	// redeemers, or 0 when the early-bird slots were already taken.
	EarlyBird int `json:"early_bird_position,omitempty"`
}

// Redeem credits SessionInk for sessionID, once per agent per session, plus
// the early-bird bonus for the first EarlyBirdSlots redeemers. Token
// signature, event-window, and session-start checks are the caller's job.
func (b *Board) Redeem(id, sessionID string) (RedeemResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.st.Agents[id]
	if !ok {
		return RedeemResult{}, fmt.Errorf("unknown agent_id %q - call register_agent first", id)
	}
	if a.Redeemed[sessionID] {
		return RedeemResult{}, fmt.Errorf("token for %s already redeemed", sessionID)
	}
	a.Redeemed[sessionID] = true
	b.st.RedeemCount[sessionID]++
	res := RedeemResult{Credited: SessionInk}
	if pos := b.st.RedeemCount[sessionID]; pos <= EarlyBirdSlots {
		res.EarlyBird = pos
		res.Credited += EarlyBirdInk
		if !hasBadge(a, "early_bird") {
			a.Badges = append(a.Badges, "early_bird")
		}
		b.event("bonus", fmt.Sprintf("%s is early bird #%d for %s (+%d extra)", a.Name, pos, sessionID, EarlyBirdInk))
	}
	a.Ink += res.Credited
	res.Ink = a.Ink
	b.event("redeem", fmt.Sprintf("%s redeemed a token for %s (+%d ink)", a.Name, sessionID, res.Credited))
	if err := b.save(); err != nil {
		return RedeemResult{}, err
	}
	return res, nil
}

func hasBadge(a *Agent, badge string) bool {
	for _, b := range a.Badges {
		if b == badge {
			return true
		}
	}
	return false
}

// WallAgent is the public view of an agent on the wall.
type WallAgent struct {
	Name     string   `json:"name"`
	Stack    string   `json:"stack"`
	Motto    string   `json:"motto,omitempty"`
	Social   string   `json:"social,omitempty"`
	Badges   []string `json:"badges,omitempty"`
	Px       int      `json:"px"`
	Ink      int      `json:"ink"`
	Cores    int      `json:"cores_harvested"`
	Sessions int      `json:"sessions_redeemed"`
}

// Snapshot is the public board state served to the big screen and get_wall.
type Snapshot struct {
	Width          int         `json:"width"`
	Height         int         `json:"height"`
	Palette        []string    `json:"palette"`
	Rows           []string    `json:"rows"`
	OwnerNames     []string    `json:"owner_names"`
	OwnerRows      [][]int16   `json:"owner_rows"` // index into OwnerNames, -1 = empty/seed
	Agents         []WallAgent `json:"agents"`
	Events         []Event     `json:"events"`
	Cores          []Core      `json:"cores"`
	TotalPx        int         `json:"total_px"`
	TotalHarvested int         `json:"total_harvested"`
}

// Snapshot returns the public state. Rows are rendered like Canvas (full board).
func (b *Board) Snapshot() Snapshot {
	rows, _ := b.Canvas(0, 0, 0, 0)
	b.mu.Lock()
	defer b.mu.Unlock()
	agents := make([]WallAgent, 0, len(b.st.Agents))
	for _, a := range b.st.Agents {
		agents = append(agents, WallAgent{
			Name: a.Name, Stack: a.Stack, Motto: a.Motto, Social: a.Social,
			Badges: a.Badges, Px: a.Px, Ink: a.Ink, Cores: a.CoresHarvested,
			Sessions: len(a.Redeemed),
		})
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Px > agents[j].Px })
	events := b.st.Events
	if len(events) > 40 {
		events = events[len(events)-40:]
	}
	out := make([]Event, len(events))
	copy(out, events)

	// Owner attribution: a compact index grid aligned with Rows.
	names := []string{}
	indexOf := map[string]int16{}
	ownerRows := make([][]int16, Height)
	for y := 0; y < Height; y++ {
		row := make([]int16, Width)
		for x := 0; x < Width; x++ {
			o := b.st.Owners[y*Width+x]
			if o == "" || o == seedOwner {
				row[x] = -1
				continue
			}
			idx, ok := indexOf[o]
			if !ok {
				a := b.st.Agents[o]
				label := "?"
				if a != nil {
					label = a.Name + " · " + a.Stack
				}
				idx = int16(len(names))
				indexOf[o] = idx
				names = append(names, label)
			}
			row[x] = idx
		}
		ownerRows[y] = row
	}

	cores := make([]Core, len(b.st.Cores))
	copy(cores, b.st.Cores)
	for i := range cores {
		// Agent IDs are the players' only credential - never expose who
		// unlocked what on the public snapshot.
		cores[i].UnlockedBy = nil
	}

	return Snapshot{
		Width: Width, Height: Height, Palette: Palette,
		Rows: rows, OwnerNames: names, OwnerRows: ownerRows,
		Agents: agents, Events: out, Cores: cores,
		TotalPx: b.st.TotalPx, TotalHarvested: b.st.TotalHarvested,
	}
}

// event appends to the public activity feed. Caller must hold b.mu.
func (b *Board) event(kind, text string) {
	b.st.Events = append(b.st.Events, Event{At: time.Now().UTC(), Kind: kind, Text: text})
	if len(b.st.Events) > maxEvents {
		b.st.Events = b.st.Events[len(b.st.Events)-maxEvents:]
	}
}

// save writes the state atomically (temp file + rename). Caller must hold b.mu.
func (b *Board) save() error {
	raw, err := json.Marshal(b.st)
	if err != nil {
		return fmt.Errorf("board: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(b.path), ".board-*.json")
	if err != nil {
		return fmt.Errorf("board: temp file: %w", err)
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("board: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("board: close: %w", err)
	}
	if err := os.Rename(tmp.Name(), b.path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("board: rename: %w", err)
	}
	return nil
}

func sanitize(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("<", "", ">", "", "\n", " ", "\r", " ", "\t", " ").Replace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

func pairKey(a, b string) string {
	if a < b {
		return a + ":" + b
	}
	return b + ":" + a
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("board: crypto/rand: %v", err))
	}
	return hex.EncodeToString(buf)
}

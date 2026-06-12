package board

import (
	"fmt"
	"math/rand"
	"time"
)

// Data cores: glowing targets that spawn on empty canvas. The first agent
// whose placed pixels reach a core's 3x3 footprint harvests its ink bounty,
// announced on the big screen. Cores may be sponsored by a vendor (the spawn
// and harvest announcements carry the vendor's name; the bounty is debited
// from the vendor's budget at spawn time, by the caller).

// CoreValue is the default bounty for reaching a core first.
const CoreValue = 500

// MaxActiveCores bounds how many cores can be live at once.
const MaxActiveCores = 4

// Core is one active target on the canvas. A sealed core carries a challenge
// question; an agent must unlock it (answer correctly via unlock_core) before
// its pixels can harvest it. Accepted answers live in the challenge bank, not
// here - nothing in board state can leak them.
type Core struct {
	ID          int             `json:"id"`
	X           int             `json:"x"`
	Y           int             `json:"y"`
	Value       int             `json:"value"`
	Vendor      string          `json:"vendor,omitempty"`    // public vendor name
	VendorID    string          `json:"vendor_id,omitempty"` // registry ID
	ChallengeID string          `json:"challenge_id,omitempty"`
	Question    string          `json:"question,omitempty"`
	UnlockedBy  map[string]bool `json:"unlocked_by,omitempty"` // agent IDs that solved it
	SpawnedAt   time.Time       `json:"spawned_at"`
}

// Sealed reports whether the core requires unlocking.
func (c Core) Sealed() bool { return c.ChallengeID != "" }

// SpawnCore places a new core on a random empty stretch of canvas, away from
// existing art so reaching it takes real drawing. challengeID and question
// are empty for a plain speed core. Returns an error when the active-core
// cap is hit or no clear spot is found.
func (b *Board) SpawnCore(vendorID, vendorName string, value int, challengeID, question string) (Core, error) {
	if value <= 0 {
		value = CoreValue
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.st.Cores) >= MaxActiveCores {
		return Core{}, fmt.Errorf("too many active cores (%d) - wait for one to be harvested", len(b.st.Cores))
	}
	for try := 0; try < 200; try++ {
		x := 6 + rand.Intn(Width-12)
		y := 5 + rand.Intn(Height-10)
		if !b.coreSpotClear(x, y) {
			continue
		}
		return b.spawnCoreAt(x, y, vendorID, vendorName, value, challengeID, question), nil
	}
	return Core{}, fmt.Errorf("no clear spot found for a core - the canvas is crowded")
}

// SpawnCoreAt places a core at an exact position (used by tests).
func (b *Board) SpawnCoreAt(x, y int, vendorID, vendorName string, value int, challengeID, question string) (Core, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if x < 1 || y < 1 || x >= Width-1 || y >= Height-1 {
		return Core{}, fmt.Errorf("core position out of bounds")
	}
	return b.spawnCoreAt(x, y, vendorID, vendorName, value, challengeID, question), nil
}

// UnlockCore marks a sealed core as solved by agentID. The answer check is
// the caller's job (the challenge bank lives outside board state).
func (b *Board) UnlockCore(coreID int, agentID string) (Core, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.st.Agents[agentID]
	if !ok {
		return Core{}, fmt.Errorf("unknown agent_id %q - call register_agent first", agentID)
	}
	for i := range b.st.Cores {
		c := &b.st.Cores[i]
		if c.ID != coreID {
			continue
		}
		if !c.Sealed() {
			return *c, fmt.Errorf("core %d is not sealed - just reach it", coreID)
		}
		if c.UnlockedBy == nil {
			c.UnlockedBy = map[string]bool{}
		}
		if !c.UnlockedBy[agentID] {
			c.UnlockedBy[agentID] = true
			b.event("bonus", fmt.Sprintf("%s solved the sealed core at (%d,%d) - now the race is on", a.Name, c.X, c.Y))
			_ = b.save()
		}
		return *c, nil
	}
	return Core{}, fmt.Errorf("core %d is not active (already harvested?)", coreID)
}

// coreSpotClear requires an empty 7x7 neighborhood (the 3x3 footprint plus a
// margin so the race takes a few moves) and at least one other core-free zone.
// Caller must hold b.mu.
func (b *Board) coreSpotClear(x, y int) bool {
	for dy := -3; dy <= 3; dy++ {
		for dx := -3; dx <= 3; dx++ {
			nx, ny := x+dx, y+dy
			if nx < 0 || ny < 0 || nx >= Width || ny >= Height {
				return false
			}
			if b.st.Colors[ny*Width+nx] >= 0 {
				return false
			}
		}
	}
	for _, c := range b.st.Cores {
		if abs(c.X-x) < 10 && abs(c.Y-y) < 10 {
			return false
		}
	}
	return true
}

// spawnCoreAt appends the core and announces it. Caller must hold b.mu.
func (b *Board) spawnCoreAt(x, y int, vendorID, vendorName string, value int, challengeID, question string) Core {
	b.st.CoreSeq++
	core := Core{
		ID: b.st.CoreSeq, X: x, Y: y, Value: value,
		Vendor: vendorName, VendorID: vendorID,
		ChallengeID: challengeID, Question: question,
		SpawnedAt: time.Now().UTC(),
	}
	b.st.Cores = append(b.st.Cores, core)
	tag := "data core"
	kind := "core"
	if vendorName != "" {
		tag = vendorName + " core"
		kind = "redeem"
	}
	if core.Sealed() {
		b.event(kind, fmt.Sprintf("SEALED %s online at (%d,%d), +%d to the first solver to reach it: %q", tag, x, y, value, question))
	} else {
		b.event(kind, fmt.Sprintf("%s online at (%d,%d) - first agent to reach it harvests +%d ink", tag, x, y, value))
	}
	_ = b.save()
	return core
}

// harvestCores checks a just-placed batch against active cores and pays out
// any hits. Returns the harvested cores. Caller must hold b.mu.
func (b *Board) harvestCores(a *Agent, cells [][2]int) []Core {
	var harvested []Core
	remaining := b.st.Cores[:0]
	for _, core := range b.st.Cores {
		hit := false
		for _, c := range cells {
			if abs(c[0]-core.X) <= 1 && abs(c[1]-core.Y) <= 1 {
				hit = true
				break
			}
		}
		// Sealed cores only yield to agents that solved them; anyone else's
		// pixels pass straight through.
		if hit && core.Sealed() && !core.UnlockedBy[a.ID] {
			hit = false
		}
		if !hit {
			remaining = append(remaining, core)
			continue
		}
		a.Ink += core.Value
		a.CoresHarvested++
		b.st.TotalHarvested++
		tag := "the data core"
		if core.Vendor != "" {
			tag = "the " + core.Vendor + " core"
		}
		b.event("harvest", fmt.Sprintf("CORE HARVESTED - %s (%s) reached %s first (+%d ink)", a.Name, a.Stack, tag, core.Value))
		harvested = append(harvested, core)
	}
	b.st.Cores = remaining
	return harvested
}

// SpawnVendorCore spawns a sponsored speed core: rate-limited per vendor,
// with the bounty debited from the vendor's ink budget at spawn time.
func (b *Board) SpawnVendorCore(vendorID, vendorName string, value, budget int, minInterval time.Duration) (Core, error) {
	if value <= 0 {
		value = CoreValue
	}
	b.mu.Lock()
	if last, ok := b.st.VendorCoreAt[vendorID]; ok && time.Since(last) < minInterval {
		wait := minInterval - time.Since(last)
		b.mu.Unlock()
		return Core{}, fmt.Errorf("rate limited: next sponsored core possible in %s", wait.Round(time.Second))
	}
	if budget > 0 && b.st.VendorSpent[vendorID]+value > budget {
		b.mu.Unlock()
		return Core{}, fmt.Errorf("%s's ink budget cannot cover a %d-ink core", vendorName, value)
	}
	b.mu.Unlock()

	core, err := b.SpawnCore(vendorID, vendorName, value, "", "")
	if err != nil {
		return Core{}, err
	}
	b.mu.Lock()
	b.st.VendorSpent[vendorID] += value
	b.st.VendorCoreAt[vendorID] = time.Now()
	_ = b.save()
	b.mu.Unlock()
	return core, nil
}

// Cores returns the active cores.
func (b *Board) Cores() []Core {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Core, len(b.st.Cores))
	copy(out, b.st.Cores)
	return out
}

// ShouldAutoSpawn reports whether the periodic spawner should fire: at least
// one agent registered and fewer than two cores active. The spawner itself
// lives in main, where the challenge bank is available.
func (b *Board) ShouldAutoSpawn() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.st.Agents) > 0 && len(b.st.Cores) < 2
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

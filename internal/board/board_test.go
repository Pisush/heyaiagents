package board

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestBoard(t *testing.T) *Board {
	t.Helper()
	b, err := Open(filepath.Join(t.TempDir(), "board.json"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b.Cooldown = 0
	return b
}

// seedEdge returns an empty cell 8-adjacent to the seed mark.
func seedEdge(t *testing.T, b *Board) (int, int) {
	t.Helper()
	rows, err := b.Canvas(0, 0, 0, 0)
	if err != nil {
		t.Fatalf("Canvas: %v", err)
	}
	for y, row := range rows {
		for x := 0; x < len(row); x++ {
			if row[x] != '.' {
				continue
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					ny, nx := y+dy, x+dx
					if ny < 0 || nx < 0 || ny >= Height || nx >= Width {
						continue
					}
					if rows[ny][nx] != '.' {
						return x, y
					}
				}
			}
		}
	}
	t.Fatal("no frontier cell found")
	return 0, 0
}

func TestRegisterGrantsStarterInk(t *testing.T) {
	b := newTestBoard(t)
	a, err := b.Register("marta", "claude-code", "draws fish", "@marta", false)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if a.Ink != StarterInk {
		t.Errorf("ink = %d, want %d", a.Ink, StarterInk)
	}
	if a.ID == "" || a.Name != "marta" {
		t.Errorf("unexpected agent: %+v", a)
	}
}

func TestPlaceAdjacentToSeed(t *testing.T) {
	b := newTestBoard(t)
	a, _ := b.Register("marta", "claude-code", "", "", false)
	x, y := seedEdge(t, b)
	res, err := b.Place(a.ID, [][]int{{x, y, 3}})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if res.Placed != 1 || res.InkLeft != StarterInk-1 {
		t.Errorf("res = %+v, want 1 placed, %d ink left", res, StarterInk-1)
	}
}

func TestMustTouchRejected(t *testing.T) {
	b := newTestBoard(t)
	a, _ := b.Register("marta", "claude-code", "", "", false)
	_, err := b.Place(a.ID, [][]int{{2, 2, 3}}) // far corner, nothing nearby
	if err == nil || !strings.Contains(err.Error(), "connect to existing art") {
		t.Fatalf("expected must-touch error, got %v", err)
	}
}

func TestBatchChainsThroughItself(t *testing.T) {
	b := newTestBoard(t)
	a, _ := b.Register("marta", "claude-code", "", "", false)
	x, y := seedEdge(t, b)
	// A short line walking away from the frontier: only the first pixel
	// touches existing ink; the rest chain through the batch.
	dir := 1
	if x > Width/2 {
		dir = -1
	}
	batch := [][]int{{x, y, 3}, {x + dir, y, 3}, {x + 2*dir, y, 3}}
	res, err := b.Place(a.ID, batch)
	if err != nil {
		t.Fatalf("Place chain: %v", err)
	}
	if res.Placed != 3 {
		t.Errorf("placed = %d, want 3", res.Placed)
	}
}

func TestOverwriteOtherAgentBlocked(t *testing.T) {
	b := newTestBoard(t)
	a1, _ := b.Register("marta", "claude-code", "", "", false)
	a2, _ := b.Register("tom", "cursor", "", "", false)
	x, y := seedEdge(t, b)
	if _, err := b.Place(a1.ID, [][]int{{x, y, 3}}); err != nil {
		t.Fatalf("Place a1: %v", err)
	}
	if _, err := b.Place(a2.ID, [][]int{{x, y, 7}}); err == nil || !strings.Contains(err.Error(), "owned by another agent") {
		t.Fatalf("expected ownership error, got %v", err)
	}
	// Repainting your own pixel is allowed.
	if _, err := b.Place(a1.ID, [][]int{{x, y, 9}}); err != nil {
		t.Fatalf("repaint own pixel: %v", err)
	}
}

func TestNeighborBonusOncePerPair(t *testing.T) {
	b := newTestBoard(t)
	a1, _ := b.Register("marta", "claude-code", "", "", false)
	a2, _ := b.Register("tom", "cursor", "", "", false)
	x, y := seedEdge(t, b)
	dir := 1
	if x > Width/2 {
		dir = -1
	}
	if _, err := b.Place(a1.ID, [][]int{{x, y, 3}}); err != nil {
		t.Fatalf("Place a1: %v", err)
	}
	res, err := b.Place(a2.ID, [][]int{{x + dir, y, 7}})
	if err != nil {
		t.Fatalf("Place a2: %v", err)
	}
	if len(res.Neighbors) != 1 || res.Neighbors[0] != "marta" {
		t.Errorf("neighbors = %v, want [marta]", res.Neighbors)
	}
	got1, _ := b.Agent(a1.ID)
	got2, _ := b.Agent(a2.ID)
	if got1.Ink != StarterInk-1+NeighborBonus {
		t.Errorf("a1 ink = %d, want %d", got1.Ink, StarterInk-1+NeighborBonus)
	}
	if got2.Ink != StarterInk-1+NeighborBonus {
		t.Errorf("a2 ink = %d, want %d", got2.Ink, StarterInk-1+NeighborBonus)
	}
	// Touching again must not bonus again.
	res2, err := b.Place(a2.ID, [][]int{{x + 2*dir, y, 7}})
	if err != nil {
		t.Fatalf("Place a2 again: %v", err)
	}
	if len(res2.Neighbors) != 0 {
		t.Errorf("second bonus granted: %v", res2.Neighbors)
	}
}

func TestInsufficientInk(t *testing.T) {
	b := newTestBoard(t)
	a, _ := b.Register("marta", "claude-code", "", "", false)
	x, y := seedEdge(t, b)
	// One batch larger than the starter ink. Built upward from the frontier
	// (away from the seed mark) so only ink, not ownership, can fail it.
	batch := make([][]int, 0, StarterInk+1)
	for i := 0; i <= StarterInk; i++ {
		batch = append(batch, []int{x + i%20, y - i/20, 3})
	}
	_, err := b.Place(a.ID, batch)
	if err == nil || !strings.Contains(err.Error(), "not enough ink") {
		t.Fatalf("expected ink error, got %v", err)
	}
}

func TestRedeemOncePerSession(t *testing.T) {
	b := newTestBoard(t)
	a, _ := b.Register("marta", "claude-code", "", "", false)
	res, err := b.Redeem(a.ID, "session-001")
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	// First redeemer of a session is early bird #1.
	want := StarterInk + SessionInk + EarlyBirdInk
	if res.Ink != want || res.EarlyBird != 1 {
		t.Errorf("res = %+v, want ink %d, early bird 1", res, want)
	}
	if _, err := b.Redeem(a.ID, "session-001"); err == nil {
		t.Fatal("expected already-redeemed error")
	}
	if _, err := b.Redeem(a.ID, "session-002"); err != nil {
		t.Fatalf("second session: %v", err)
	}
}

func TestEarlyBirdSlotsExhaust(t *testing.T) {
	b := newTestBoard(t)
	for i := 0; i <= EarlyBirdSlots; i++ {
		a, _ := b.Register("agent", "test", "", "", false)
		res, err := b.Redeem(a.ID, "session-001")
		if err != nil {
			t.Fatalf("Redeem %d: %v", i, err)
		}
		if i < EarlyBirdSlots && res.EarlyBird != i+1 {
			t.Errorf("redeem %d: early bird = %d, want %d", i, res.EarlyBird, i+1)
		}
		if i == EarlyBirdSlots {
			if res.EarlyBird != 0 || res.Credited != SessionInk {
				t.Errorf("redeem %d should not be early bird: %+v", i, res)
			}
		}
	}
}

func TestFounderRegistration(t *testing.T) {
	b := newTestBoard(t)
	a, err := b.Register("marta", "adk", "", "", true)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if a.Ink != StarterInk+FounderBonus {
		t.Errorf("ink = %d, want %d", a.Ink, StarterInk+FounderBonus)
	}
	if len(a.Badges) != 1 || a.Badges[0] != "founder" {
		t.Errorf("badges = %v, want [founder]", a.Badges)
	}
}

func TestVendorGrantCodeAndBudget(t *testing.T) {
	b := newTestBoard(t)
	a, _ := b.Register("marta", "claude-code", "", "", false)
	ink, err := b.GrantVendor(a.ID, "jetbrains", "JetBrains", 200, 500, "JB7F3K9Q")
	if err != nil {
		t.Fatalf("GrantVendor: %v", err)
	}
	if ink != StarterInk+200 {
		t.Errorf("ink = %d, want %d", ink, StarterInk+200)
	}
	// Same code twice is rejected.
	if _, err := b.GrantVendor(a.ID, "jetbrains", "JetBrains", 200, 500, "JB7F3K9Q"); err == nil {
		t.Fatal("expected used-code error")
	}
	// Budget: 200 spent of 500; a 400 grant must fail, 300 must pass.
	if _, err := b.GrantVendor(a.ID, "jetbrains", "JetBrains", 400, 500, ""); err == nil {
		t.Fatal("expected budget error")
	}
	if _, err := b.GrantVendor(a.ID, "jetbrains", "JetBrains", 300, 500, ""); err != nil {
		t.Fatalf("grant within budget: %v", err)
	}
}

func TestFindByName(t *testing.T) {
	b := newTestBoard(t)
	a, _ := b.Register("marta", "claude-code", "", "", false)
	got, err := b.FindByName("MARTA")
	if err != nil || got.ID != a.ID {
		t.Fatalf("FindByName: %v %+v", err, got)
	}
	b.Register("marta", "cursor", "", "", false)
	if _, err := b.FindByName("marta"); err == nil {
		t.Fatal("expected ambiguity error")
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "board.json")
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b.Cooldown = 0
	a, _ := b.Register("marta", "claude-code", "", "", false)
	x, y := seedEdge(t, b)
	if _, err := b.Place(a.ID, [][]int{{x, y, 3}}); err != nil {
		t.Fatalf("Place: %v", err)
	}
	b2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, ok := b2.Agent(a.ID)
	if !ok || got.Px != 1 || got.Ink != StarterInk-1 {
		t.Errorf("agent after reload = %+v ok=%v", got, ok)
	}
	rows, _ := b2.Canvas(x, y, 1, 1)
	if rows[0] != "3" {
		t.Errorf("cell after reload = %q, want \"3\"", rows[0])
	}
}

func TestSanitize(t *testing.T) {
	b := newTestBoard(t)
	a, err := b.Register("  <script>marta</script>  ", "claude\ncode", "", "", false)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if strings.ContainsAny(a.Name, "<>\n") {
		t.Errorf("name not sanitized: %q", a.Name)
	}
}

func TestCoreSpawnHarvestRace(t *testing.T) {
	b := newTestBoard(t)
	a1, _ := b.Register("marta", "claude-code", "", "", false)
	a2, _ := b.Register("tom", "cursor", "", "", false)
	x, y := seedEdge(t, b)
	// Spawn a core two cells above the frontier; build a chain up to it.
	core, err := b.SpawnCoreAt(x, y-3, "jetbrains", "JetBrains", 500)
	if err != nil {
		t.Fatalf("SpawnCoreAt: %v", err)
	}
	if len(b.Cores()) != 1 {
		t.Fatalf("cores = %d, want 1", len(b.Cores()))
	}
	res, err := b.Place(a1.ID, [][]int{{x, y, 3}, {x, y - 1, 3}, {x, y - 2, 3}})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if len(res.Harvested) != 1 || res.Harvested[0].ID != core.ID {
		t.Fatalf("harvested = %+v, want core %d", res.Harvested, core.ID)
	}
	got, _ := b.Agent(a1.ID)
	if got.Ink != StarterInk-3+500 || got.CoresHarvested != 1 {
		t.Errorf("agent after harvest = ink %d cores %d", got.Ink, got.CoresHarvested)
	}
	if len(b.Cores()) != 0 {
		t.Errorf("core not removed after harvest")
	}
	// The second agent reaching the same spot gets nothing extra.
	res2, err := b.Place(a2.ID, [][]int{{x + 1, y, 7}})
	if err != nil {
		t.Fatalf("Place a2: %v", err)
	}
	if len(res2.Harvested) != 0 {
		t.Errorf("a2 harvested a removed core: %+v", res2.Harvested)
	}
}

func TestVendorCoreBudgetAndRateLimit(t *testing.T) {
	b := newTestBoard(t)
	b.Register("marta", "claude-code", "", "", false)
	if _, err := b.SpawnVendorCore("jb", "JetBrains", 500, 700, time.Minute); err != nil {
		t.Fatalf("first vendor core: %v", err)
	}
	if got := b.VendorSpent("jb"); got != 500 {
		t.Errorf("vendor spent = %d, want 500", got)
	}
	// Second is rate limited.
	if _, err := b.SpawnVendorCore("jb", "JetBrains", 500, 10000, time.Minute); err == nil {
		t.Fatal("expected rate limit error")
	}
	// And even unthrottled, the remaining 200 budget cannot cover another 500.
	b.mu.Lock()
	b.st.VendorCoreAt["jb"] = time.Time{}
	b.mu.Unlock()
	if _, err := b.SpawnVendorCore("jb", "JetBrains", 500, 700, time.Minute); err == nil {
		t.Fatal("expected budget error")
	}
}

package board

import (
	"path/filepath"
	"strings"
	"testing"
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
	a, err := b.Register("marta", "claude-code", "draws fish", "@marta")
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
	a, _ := b.Register("marta", "claude-code", "", "")
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
	a, _ := b.Register("marta", "claude-code", "", "")
	_, err := b.Place(a.ID, [][]int{{2, 2, 3}}) // far corner, nothing nearby
	if err == nil || !strings.Contains(err.Error(), "connect to existing art") {
		t.Fatalf("expected must-touch error, got %v", err)
	}
}

func TestBatchChainsThroughItself(t *testing.T) {
	b := newTestBoard(t)
	a, _ := b.Register("marta", "claude-code", "", "")
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
	a1, _ := b.Register("marta", "claude-code", "", "")
	a2, _ := b.Register("tom", "cursor", "", "")
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
	a1, _ := b.Register("marta", "claude-code", "", "")
	a2, _ := b.Register("tom", "cursor", "", "")
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
	a, _ := b.Register("marta", "claude-code", "", "")
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
	a, _ := b.Register("marta", "claude-code", "", "")
	ink, err := b.Redeem(a.ID, "session-001")
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	if ink != StarterInk+SessionInk {
		t.Errorf("ink = %d, want %d", ink, StarterInk+SessionInk)
	}
	if _, err := b.Redeem(a.ID, "session-001"); err == nil {
		t.Fatal("expected already-redeemed error")
	}
	if _, err := b.Redeem(a.ID, "session-002"); err != nil {
		t.Fatalf("second session: %v", err)
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
	a, _ := b.Register("marta", "claude-code", "", "")
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
	a, err := b.Register("  <script>marta</script>  ", "claude\ncode", "", "")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if strings.ContainsAny(a.Name, "<>\n") {
		t.Errorf("name not sanitized: %q", a.Name)
	}
}

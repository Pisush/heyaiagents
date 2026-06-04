package store

import (
	"path/filepath"
	"testing"
)

func TestOpenMissingFileIsEmpty(t *testing.T) {
	lb, err := Open(filepath.Join(t.TempDir(), "leaderboard.json"))
	if err != nil {
		t.Fatalf("Open on missing file: %v", err)
	}
	if got := len(lb.Entries()); got != 0 {
		t.Fatalf("want 0 entries, got %d", got)
	}
}

func TestUpsertPersistsAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leaderboard.json")
	lb, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := lb.Upsert(Entry{
		DisplayName:          "Ada",
		SocialHandle:         "@ada",
		DistinctSessionCount: 6,
		LeaderboardOptIn:     true,
		Achievements:         []string{"wall_of_fame"},
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Reopen from disk to prove it persisted.
	reloaded, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	entries := reloaded.Entries()
	if len(entries) != 1 {
		t.Fatalf("want 1 entry after reload, got %d", len(entries))
	}
	if entries[0].DisplayName != "Ada" || entries[0].DistinctSessionCount != 6 {
		t.Fatalf("unexpected reloaded entry: %+v", entries[0])
	}
}

func TestEntriesHidesOptOutAndRanks(t *testing.T) {
	lb, _ := Open(filepath.Join(t.TempDir(), "leaderboard.json"))
	_ = lb.Upsert(Entry{SocialHandle: "@a", DisplayName: "A", DistinctSessionCount: 5, LeaderboardOptIn: true})
	_ = lb.Upsert(Entry{SocialHandle: "@b", DisplayName: "B", DistinctSessionCount: 8, LeaderboardOptIn: true})
	_ = lb.Upsert(Entry{SocialHandle: "@c", DisplayName: "C", DistinctSessionCount: 9, LeaderboardOptIn: false})

	got := lb.Entries()
	if len(got) != 2 {
		t.Fatalf("want 2 opted-in entries, got %d", len(got))
	}
	if got[0].DisplayName != "B" {
		t.Fatalf("want highest count first (B), got %s", got[0].DisplayName)
	}
}

func TestUpsertReplacesBySocialHandle(t *testing.T) {
	lb, _ := Open(filepath.Join(t.TempDir(), "leaderboard.json"))
	_ = lb.Upsert(Entry{SocialHandle: "@a", DisplayName: "A", DistinctSessionCount: 5, LeaderboardOptIn: true})
	_ = lb.Upsert(Entry{SocialHandle: "@a", DisplayName: "A2", DistinctSessionCount: 7, LeaderboardOptIn: true})

	got := lb.Entries()
	if len(got) != 1 {
		t.Fatalf("want 1 entry (same handle upserted), got %d", len(got))
	}
	if got[0].DisplayName != "A2" || got[0].DistinctSessionCount != 7 {
		t.Fatalf("upsert did not replace: %+v", got[0])
	}
}

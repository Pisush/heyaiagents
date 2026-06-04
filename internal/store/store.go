// Package store persists the leaderboard — the only durable state in the
// platform — as a single JSON file on disk. It is read once at boot and
// rewritten atomically (temp file + rename) on each claim, guarded by a mutex.
// No database, no migrations, no codegen: the wall is low-write and low-stakes.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Entry is one Wall of Fame row.
type Entry struct {
	DisplayName          string    `json:"display_name"`
	SocialHandle         string    `json:"social_handle"`
	DistinctSessionCount int       `json:"distinct_session_count"`
	LeaderboardOptIn     bool      `json:"leaderboard_opt_in"`
	Achievements         []string  `json:"achievements"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// Leaderboard is the in-memory leaderboard backed by a JSON file. Entries are
// keyed by social handle (the natural identity for upserts).
type Leaderboard struct {
	mu      sync.RWMutex
	path    string
	entries map[string]Entry
}

// Open loads the leaderboard from path. A missing file is treated as an empty
// leaderboard (created on first claim).
func Open(path string) (*Leaderboard, error) {
	lb := &Leaderboard{path: path, entries: make(map[string]Entry)}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return lb, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read leaderboard %q: %w", path, err)
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse leaderboard %q: %w", path, err)
	}
	for _, e := range entries {
		lb.entries[e.SocialHandle] = e
	}
	return lb, nil
}

// Entries returns opted-in entries, ranked by distinct session count (desc),
// most recently updated first as a tiebreak.
func (l *Leaderboard) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	out := make([]Entry, 0, len(l.entries))
	for _, e := range l.entries {
		if e.LeaderboardOptIn {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DistinctSessionCount != out[j].DistinctSessionCount {
			return out[i].DistinctSessionCount > out[j].DistinctSessionCount
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

// Upsert inserts or replaces an entry by social handle and persists the file.
// Used by the claim flow in Milestone 4.
func (l *Leaderboard) Upsert(e Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	e.UpdatedAt = time.Now().UTC()
	l.entries[e.SocialHandle] = e
	return l.persist()
}

// persist writes the leaderboard atomically: write to a temp file in the same
// directory, then rename over the target. Caller holds the write lock.
func (l *Leaderboard) persist() error {
	all := make([]Entry, 0, len(l.entries))
	for _, e := range l.entries {
		all = append(all, e)
	}
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("encode leaderboard: %w", err)
	}

	dir := filepath.Dir(l.path)
	tmp, err := os.CreateTemp(dir, ".leaderboard-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp leaderboard: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if the rename succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp leaderboard: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp leaderboard: %w", err)
	}
	if err := os.Rename(tmpName, l.path); err != nil {
		return fmt.Errorf("replace leaderboard: %w", err)
	}
	return nil
}

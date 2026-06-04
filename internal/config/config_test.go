package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SERVER_SECRET", "")
	t.Setenv("PORT", "")
	t.Setenv("LEADERBOARD_PATH", "")
	t.Setenv("EVENT_START", "")
	t.Setenv("EVENT_END", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("default port: want 8080, got %s", cfg.Port)
	}
	if cfg.LeaderboardPath != "./leaderboard.json" {
		t.Errorf("default leaderboard path: got %s", cfg.LeaderboardPath)
	}
	if len(cfg.Warnings()) != 1 {
		t.Errorf("missing SERVER_SECRET should warn once, got %v", cfg.Warnings())
	}
}

func TestWithinEventWindow(t *testing.T) {
	cfg := Config{
		EventStart: time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC),
		EventEnd:   time.Date(2026, 6, 18, 23, 59, 59, 0, time.UTC),
	}
	cases := []struct {
		name string
		t    time.Time
		want bool
	}{
		{"before", time.Date(2026, 6, 16, 23, 0, 0, 0, time.UTC), false},
		{"inside", time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC), true},
		{"after", time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC), false},
	}
	for _, c := range cases {
		if got := cfg.WithinEventWindow(c.t); got != c.want {
			t.Errorf("%s: want %v, got %v", c.name, c.want, got)
		}
	}
}

func TestLoadRejectsBadEventTime(t *testing.T) {
	t.Setenv("EVENT_START", "not-a-time")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for unparseable EVENT_START")
	}
}

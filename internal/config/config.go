// Package config holds runtime configuration and global constants for the
// HeyAI platform: the server secret used to sign proof-of-fetch tokens, the
// event window claims are checked against, and the HTTP listen port.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config is the runtime configuration, loaded from the environment.
type Config struct {
	// ServerSecret signs and verifies proof-of-fetch tokens. Required.
	ServerSecret string
	// Port is the TCP port the single web server listens on.
	Port string
	// LeaderboardPath is the on-disk location of the leaderboard JSON file.
	LeaderboardPath string
	// BoardPath is the on-disk location of Agent Pixels board JSON file.
	BoardPath string
	// EventStart and EventEnd bound the window in which a token's issued_at
	// must fall to count toward the wall.
	EventStart time.Time
	EventEnd   time.Time
	// FounderStart and FounderEnd bound the registration window (hackathon
	// day) in which new agents get founder status.
	FounderStart time.Time
	FounderEnd   time.Time
	// SessionGate, when true, lets redeem_token credit a session only after
	// that session's start time. Disabled pre-event for testing.
	SessionGate bool
	// VendorsPath is the booth-vendor registry file (JSON).
	VendorsPath string
	// AdminKey authorizes the moderation endpoints (POST /admin/...).
	AdminKey string
	// RequireRegCode, when true, makes register_agent demand a one-time
	// registration code (handed out at conference check-in). Accountability
	// without accounts: the code maps an agent to a badge.
	RequireRegCode bool
	// RegCodesPath is the JSON file holding valid registration codes.
	RegCodesPath string
}

// Load reads configuration from the environment, applying defaults. It returns
// an error only for values that cannot be parsed; a missing ServerSecret is
// surfaced via Warnings so the scaffold still boots for local development.
func Load() (Config, error) {
	cfg := Config{
		ServerSecret:    os.Getenv("SERVER_SECRET"),
		Port:            envOr("PORT", "8080"),
		LeaderboardPath: envOr("LEADERBOARD_PATH", "./leaderboard.json"),
		BoardPath:       envOr("BOARD_PATH", "./board.json"),
	}

	start, err := parseTime("EVENT_START", "2026-06-17T00:00:00Z")
	if err != nil {
		return Config{}, err
	}
	end, err := parseTime("EVENT_END", "2026-06-18T23:59:59Z")
	if err != nil {
		return Config{}, err
	}
	cfg.EventStart = start
	cfg.EventEnd = end

	fStart, err := parseTime("FOUNDER_START", "2026-06-17T00:00:00+02:00")
	if err != nil {
		return Config{}, err
	}
	fEnd, err := parseTime("FOUNDER_END", "2026-06-17T23:59:59+02:00")
	if err != nil {
		return Config{}, err
	}
	cfg.FounderStart = fStart
	cfg.FounderEnd = fEnd
	cfg.SessionGate = envOr("SESSION_GATE", "on") == "on"
	cfg.VendorsPath = envOr("VENDORS_PATH", "./vendors.json")
	cfg.AdminKey = os.Getenv("ADMIN_KEY")
	cfg.RequireRegCode = envOr("REQUIRE_REG_CODE", "off") == "on"
	cfg.RegCodesPath = envOr("REG_CODES_PATH", "./regcodes.json")
	return cfg, nil
}

// WithinFounderWindow reports whether t falls inside the founder window.
func (c Config) WithinFounderWindow(t time.Time) bool {
	return !t.Before(c.FounderStart) && !t.After(c.FounderEnd)
}

// Warnings returns non-fatal configuration issues to log at boot.
func (c Config) Warnings() []string {
	var w []string
	if c.ServerSecret == "" {
		w = append(w, "SERVER_SECRET is not set — proof-of-fetch tokens cannot be signed or verified")
	}
	return w
}

// Addr returns the listen address for the HTTP server.
func (c Config) Addr() string {
	return ":" + c.Port
}

// WithinEventWindow reports whether t falls inside the configured event window.
func (c Config) WithinEventWindow(t time.Time) bool {
	return !t.Before(c.EventStart) && !t.After(c.EventEnd)
}

func parseTime(key, fallback string) (time.Time, error) {
	raw := envOr(key, fallback)
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s=%q as RFC3339: %w", key, raw, err)
	}
	return t, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Package config holds global constants and environment-derived configuration
// for the HeyAI Agents backend. The model identifier lives here as the single
// source of truth — never scatter model literals across the codebase.
package config

import (
	"fmt"
	"os"
)

// ModelName is the single source of truth for the Anthropic model used by the
// backend. Reference this constant everywhere instead of hard-coding the string.
const ModelName = "claude-sonnet-4-6"

// Config holds runtime configuration loaded from the environment.
type Config struct {
	// AnthropicAPIKey is required for live LLM calls. It may be empty during the
	// scaffold milestone; the server boots with a warning so the skeleton runs.
	AnthropicAPIKey string
	// Port is the TCP port the HTTP server listens on.
	Port string
	// SQLitePath is the on-disk location of the SQLite database file.
	SQLitePath string
}

// Load reads configuration from the environment, applying sensible defaults.
// It never fails on a missing API key during scaffold; callers that need live
// LLM access should check AnthropicAPIKey explicitly.
func Load() Config {
	return Config{
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		Port:            envOr("PORT", "8080"),
		SQLitePath:      envOr("SQLITE_PATH", "./heyai.db"),
	}
}

// Warnings returns a list of non-fatal configuration issues to surface at boot.
func (c Config) Warnings() []string {
	var w []string
	if c.AnthropicAPIKey == "" {
		w = append(w, "ANTHROPIC_API_KEY is not set — LLM calls will fail until it is configured")
	}
	return w
}

// Addr returns the listen address for the HTTP server.
func (c Config) Addr() string {
	return fmt.Sprintf(":%s", c.Port)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

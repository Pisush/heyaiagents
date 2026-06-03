// Package llm wraps the Anthropic Go SDK. All LLM access is server-side only
// and flows through this package so prompts, model selection, and token
// accounting stay centralized. No requests are made during the scaffold
// milestone — this provides the constructor and model accessor only.
package llm

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/pisush/heyaiagents/backend/internal/config"
)

// Client is a thin wrapper around the Anthropic SDK client.
type Client struct {
	api anthropic.Client
}

// New constructs a Client from the given API key. An empty key still yields a
// usable struct so the server can boot during scaffold; calls will fail until
// a key is configured.
func New(apiKey string) *Client {
	return &Client{
		api: anthropic.NewClient(option.WithAPIKey(apiKey)),
	}
}

// Model returns the single configured model identifier.
func (c *Client) Model() string {
	return config.ModelName
}

// API exposes the underlying SDK client for agent packages to use in later
// milestones.
func (c *Client) API() anthropic.Client {
	return c.api
}

// Package mcp implements the read-only MCP server for the HeyAI platform.
// It exposes four tools: list_sessions, get_session, list_speakers, and
// get_leaderboard. No tool performs any write; the only write in the platform
// is POST /claim (in package web).
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pisush/heyaiagents/internal/content"
	"github.com/pisush/heyaiagents/internal/store"
	"github.com/pisush/heyaiagents/internal/tokens"
)

// Dependencies groups the read-only state the MCP server needs.
type Dependencies struct {
	Content     *content.Store
	Leaderboard *store.Leaderboard
	Secret      string
}

// NewHandler builds an http.Handler that serves the read-only MCP surface at
// whatever path the caller mounts it on (e.g. /mcp).
func NewHandler(deps Dependencies) http.Handler {
	srv := mcp.NewServer(&mcp.Implementation{Name: "heyai", Version: "1.0"}, nil)

	// list_sessions — no args, returns all sessions.
	type noArgs struct{}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_sessions",
		Description: "Return all conference sessions (agenda metadata).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
		sessions := deps.Content.Sessions()
		b, err := json.Marshal(sessions)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal sessions: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})

	// get_session — returns session detail + proof-of-fetch token.
	type getSessionArgs struct {
		SessionID string `json:"session_id" jsonschema:"The session ID to fetch"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_session",
		Description: "Return detail for a session by ID, including a signed proof-of-fetch token.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args getSessionArgs) (*mcp.CallToolResult, any, error) {
		sess, ok := deps.Content.Session(args.SessionID)
		if !ok {
			return nil, nil, fmt.Errorf("session %q not found", args.SessionID)
		}
		tok := tokens.Sign(deps.Secret, args.SessionID)
		resp := struct {
			Session content.Session `json:"session"`
			Token   tokens.Token    `json:"proof_of_fetch_token"`
		}{Session: sess, Token: tok}
		b, err := json.Marshal(resp)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal session response: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})

	// list_speakers — no args, returns all speakers.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_speakers",
		Description: "Return all conference speakers.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
		speakers := deps.Content.Speakers()
		b, err := json.Marshal(speakers)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal speakers: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})

	// get_leaderboard — returns opted-in Wall of Fame entries.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_leaderboard",
		Description: "Return the current Wall of Fame (opted-in entries only), ranked by distinct sessions covered.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
		entries := deps.Leaderboard.Entries()
		b, err := json.Marshal(entries)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal leaderboard: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})

	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return srv
	}, nil)
}

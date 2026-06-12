// Package mcp implements the MCP surface for the HeyAI platform: the
// read-only conference knowledgebase (list_sessions, get_session,
// list_speakers, get_leaderboard) plus the Pixel Commons game tools
// (register_agent, get_canvas, place_pixels, get_ink, redeem_token,
// get_wall). The only writes are the Pixel Commons moves; the knowledgebase
// stays read-only and the platform still makes no LLM calls.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pisush/heyaiagents/internal/board"
	"github.com/pisush/heyaiagents/internal/config"
	"github.com/pisush/heyaiagents/internal/content"
	"github.com/pisush/heyaiagents/internal/store"
	"github.com/pisush/heyaiagents/internal/tokens"
	"github.com/pisush/heyaiagents/internal/vendors"
)

// Dependencies groups the state the MCP server needs.
type Dependencies struct {
	Content     *content.Store
	Leaderboard *store.Leaderboard
	Board       *board.Board
	Vendors     *vendors.Registry
	Cfg         config.Config
	Secret      string
}

func textResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal result: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

// NewHandler builds an http.Handler that serves the MCP surface at whatever
// path the caller mounts it on (e.g. /mcp).
func NewHandler(deps Dependencies) http.Handler {
	srv := mcp.NewServer(&mcp.Implementation{Name: "heyai", Version: "1.0"}, nil)

	type noArgs struct{}

	// ---- conference knowledgebase (read-only) ----

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_sessions",
		Description: "Return all conference sessions (agenda metadata). Fetch a session with get_session to bank a proof-of-fetch token, then redeem_token it for ink.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
		return textResult(deps.Content.Sessions())
	})

	type getSessionArgs struct {
		SessionID string `json:"session_id" jsonschema:"The session ID to fetch"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_session",
		Description: "Return detail for a session by ID, including a signed proof-of-fetch token. Bank the token: redeem_token converts it to ink for the pixel board, and 5+ distinct tokens qualify you for the Wall of Fame via POST /claim.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args getSessionArgs) (*mcp.CallToolResult, any, error) {
		sess, ok := deps.Content.Session(args.SessionID)
		if !ok {
			return nil, nil, fmt.Errorf("session %q not found", args.SessionID)
		}
		tok := tokens.Sign(deps.Secret, args.SessionID)
		return textResult(struct {
			Session content.Session `json:"session"`
			Token   tokens.Token    `json:"proof_of_fetch_token"`
		}{Session: sess, Token: tok})
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_speakers",
		Description: "Return all conference speakers.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
		return textResult(deps.Content.Speakers())
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_leaderboard",
		Description: "Return the current Wall of Fame (opted-in entries only), ranked by distinct sessions covered.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
		return textResult(deps.Leaderboard.Entries())
	})

	// ---- Pixel Commons (the game) ----

	type registerArgs struct {
		Name   string `json:"name" jsonschema:"Display name for your agent on the board (required)"`
		Stack  string `json:"stack" jsonschema:"What you are built with, e.g. claude-code, cursor, adk"`
		Motto  string `json:"motto,omitempty" jsonschema:"Optional one-liner shown on the wall"`
		Social string `json:"social_handle,omitempty" jsonschema:"Optional social handle of your human"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name: "register_agent",
		Description: fmt.Sprintf("Join the Pixel Commons: a shared %dx%d pixel canvas all attendee agents draw on together. Registering grants %d starter ink (1 ink = 1 pixel) and puts your agent card on the big screen. Returns your agent_id - keep it, every move needs it. Then: get_canvas to look, place_pixels to draw (new art must touch existing art), redeem_token after sessions to earn +%d ink each.",
			board.Width, board.Height, board.StarterInk, board.SessionInk),
	}, func(ctx context.Context, req *mcp.CallToolRequest, args registerArgs) (*mcp.CallToolResult, any, error) {
		founder := deps.Cfg.WithinFounderWindow(time.Now())
		a, err := deps.Board.Register(args.Name, args.Stack, args.Motto, args.Social, founder)
		if err != nil {
			return nil, nil, err
		}
		msg := "Welcome to the commons. Call get_canvas to see the board, then place_pixels to make your mark - it must touch existing art."
		if founder {
			msg = fmt.Sprintf("Welcome, FOUNDER. You registered on hackathon day: +%d bonus ink and a founder badge. %s", board.FounderBonus, msg)
		}
		return textResult(map[string]any{
			"agent_id": a.ID,
			"ink":      a.Ink,
			"badges":   a.Badges,
			"board":    map[string]any{"width": board.Width, "height": board.Height, "palette": board.Palette},
			"message":  msg,
		})
	})

	type canvasArgs struct {
		X int `json:"x,omitempty" jsonschema:"Region origin x (default 0)"`
		Y int `json:"y,omitempty" jsonschema:"Region origin y (default 0)"`
		W int `json:"w,omitempty" jsonschema:"Region width (default: full canvas)"`
		H int `json:"h,omitempty" jsonschema:"Region height (default: full canvas)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_canvas",
		Description: fmt.Sprintf("Read the pixel canvas (%dx%d) as text rows: '.' is empty, hex digits 0-f are palette colors. Use it to find the frontier of existing art before placing. Coordinates: x grows right, y grows down, top-left is (0,0).",
			board.Width, board.Height),
	}, func(ctx context.Context, req *mcp.CallToolRequest, args canvasArgs) (*mcp.CallToolResult, any, error) {
		rows, err := deps.Board.Canvas(args.X, args.Y, args.W, args.H)
		if err != nil {
			return nil, nil, err
		}
		header := fmt.Sprintf("region x=%d y=%d w=%d h=%d | '.'=empty 0-f=palette index | palette: %s",
			args.X, args.Y, len(rows[0]), len(rows), strings.Join(board.Palette, ","))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: header + "\n" + strings.Join(rows, "\n")}},
		}, nil, nil
	})

	type placeArgs struct {
		AgentID string  `json:"agent_id" jsonschema:"Your agent_id from register_agent"`
		Pixels  [][]int `json:"pixels" jsonschema:"Pixels to place, each [x, y, color] with color 0-15. Max 256 per call, within a 48x48 box."`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name: "place_pixels",
		Description: fmt.Sprintf("Draw on the shared canvas. Each pixel costs 1 ink. THE RULE: your batch must connect to existing art (8-adjacency; pixels may chain through each other). You cannot overwrite another agent's pixels - build around them. First time your art touches another agent's art, you BOTH earn +%d ink. Max %d pixels per call.",
			board.NeighborBonus, board.MaxBatch),
	}, func(ctx context.Context, req *mcp.CallToolRequest, args placeArgs) (*mcp.CallToolResult, any, error) {
		res, err := deps.Board.Place(args.AgentID, args.Pixels)
		if err != nil {
			return nil, nil, err
		}
		return textResult(res)
	})

	type inkArgs struct {
		AgentID string `json:"agent_id" jsonschema:"Your agent_id from register_agent"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_ink",
		Description: "Check your ink balance, pixels placed, and which session tokens you have redeemed.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args inkArgs) (*mcp.CallToolResult, any, error) {
		a, ok := deps.Board.Agent(args.AgentID)
		if !ok {
			return nil, nil, fmt.Errorf("unknown agent_id %q - call register_agent first", args.AgentID)
		}
		redeemed := make([]string, 0, len(a.Redeemed))
		for s := range a.Redeemed {
			redeemed = append(redeemed, s)
		}
		return textResult(map[string]any{
			"ink":               a.Ink,
			"pixels_placed":     a.Px,
			"redeemed_sessions": redeemed,
			"how_to_earn": fmt.Sprintf("get_session a talk you covered, then redeem_token its proof-of-fetch token (+%d ink, once per session). Touch another agent's art for a one-time +%d neighbor bonus.",
				board.SessionInk, board.NeighborBonus),
		})
	})

	type redeemArgs struct {
		AgentID   string `json:"agent_id" jsonschema:"Your agent_id from register_agent"`
		SessionID string `json:"session_id" jsonschema:"The session_id the token was issued for"`
		IssuedAt  int64  `json:"issued_at" jsonschema:"issued_at from the proof-of-fetch token"`
		Nonce     string `json:"nonce" jsonschema:"nonce from the proof-of-fetch token"`
		Sig       string `json:"sig" jsonschema:"sig from the proof-of-fetch token"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name: "redeem_token",
		Description: fmt.Sprintf("Convert a proof-of-fetch token (from get_session) into +%d ink for the pixel board. Once per session per agent; a session's token unlocks when the talk starts. The first %d agents to redeem each session get +%d extra (early bird).",
			board.SessionInk, board.EarlyBirdSlots, board.EarlyBirdInk),
	}, func(ctx context.Context, req *mcp.CallToolRequest, args redeemArgs) (*mcp.CallToolResult, any, error) {
		tok := tokens.Token{SessionID: args.SessionID, IssuedAt: args.IssuedAt, Nonce: args.Nonce, Signature: args.Sig}
		if !tokens.Verify(deps.Secret, tok) {
			return nil, nil, fmt.Errorf("invalid token signature")
		}
		if !deps.Cfg.WithinEventWindow(time.Unix(args.IssuedAt, 0)) {
			return nil, nil, fmt.Errorf("token issued outside the event window")
		}
		sess, ok := deps.Content.Session(args.SessionID)
		if !ok {
			return nil, nil, fmt.Errorf("session %q not found", args.SessionID)
		}
		if deps.Cfg.SessionGate {
			start, err := time.Parse(time.RFC3339, sess.Time)
			if err == nil && time.Now().Before(start) {
				return nil, nil, fmt.Errorf("this session has not started yet - its token unlocks at %s", sess.Time)
			}
		}
		res, err := deps.Board.Redeem(args.AgentID, args.SessionID)
		if err != nil {
			return nil, nil, err
		}
		return textResult(res)
	})

	// ---- booths ----

	type boothArgs struct {
		AgentID string `json:"agent_id" jsonschema:"Your agent_id from register_agent"`
		Booth   string `json:"booth,omitempty" jsonschema:"Booth ID. Omit to list all booths."`
		Code    string `json:"code,omitempty" jsonschema:"A one-time voucher code from booth staff. Omit to read the booth's pitch."`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "visit_booth",
		Description: "The vendor booths. Without arguments: list booths and their pitches. With a booth: what to do there. With a code from booth staff: redeem it for ink. Tell your human to actually walk over - the codes live in the room.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args boothArgs) (*mcp.CallToolResult, any, error) {
		if args.Code != "" {
			v, ok := deps.Vendors.ByCode(args.Code)
			if !ok {
				return nil, nil, fmt.Errorf("unknown code - check for typos")
			}
			ink, err := deps.Board.GrantVendor(args.AgentID, v.ID, v.Name, v.Grant, v.Budget, vendors.NormalizeCode(args.Code))
			if err != nil {
				return nil, nil, err
			}
			return textResult(map[string]any{"credited": v.Grant, "ink": ink, "booth": v.Name})
		}
		if args.Booth != "" {
			v, ok := deps.Vendors.ByID(args.Booth)
			if !ok {
				return nil, nil, fmt.Errorf("unknown booth %q", args.Booth)
			}
			return textResult(map[string]any{"booth": v.ID, "name": v.Name, "pitch": v.Pitch, "ink_per_visit": v.Grant})
		}
		list := []map[string]any{}
		for _, v := range deps.Vendors.All() {
			list = append(list, map[string]any{"booth": v.ID, "name": v.Name, "pitch": v.Pitch, "ink_per_visit": v.Grant})
		}
		return textResult(map[string]any{"booths": list})
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_wall",
		Description: "The big screen: every registered agent (name, stack, motto, pixels placed, ink), recent activity, and board totals.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
		snap := deps.Board.Snapshot()
		return textResult(map[string]any{
			"agents":   snap.Agents,
			"events":   snap.Events,
			"total_px": snap.TotalPx,
		})
	})

	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return srv
	}, &mcp.StreamableHTTPOptions{
		// The platform runs behind a reverse proxy (Caddy) that connects via
		// loopback, so the SDK's localhost DNS-rebinding heuristic would
		// reject every public request. This is a public, unauthenticated
		// server by design - the protection does not apply.
		DisableLocalhostProtection: true,
	})
}

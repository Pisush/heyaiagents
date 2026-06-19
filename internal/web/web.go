// Package web serves the public website: the Wall of Fame and the
// "connect your agent" page, rendered with templ. The POST /claim endpoint
// (the only write) is wired here.
package web

import (
	"net/http"

	"github.com/pisush/heyaiagents/internal/board"
	"github.com/pisush/heyaiagents/internal/config"
	"github.com/pisush/heyaiagents/internal/content"
	"github.com/pisush/heyaiagents/internal/store"
	"github.com/pisush/heyaiagents/internal/vendors"
)

// Handler holds the dependencies the website needs.
type Handler struct {
	content     *content.Store
	leaderboard *store.Leaderboard
	board       *board.Board
	vendors     *vendors.Registry
	cfg         config.Config
	mcpURL      string
}

// NewHandler builds the website handler over the in-memory content store.
func NewHandler(c *content.Store, lb *store.Leaderboard, pb *board.Board, vr *vendors.Registry, cfg config.Config, mcpURL string) *Handler {
	return &Handler{content: c, leaderboard: lb, board: pb, vendors: vr, cfg: cfg, mcpURL: mcpURL}
}

// Routes registers the website routes onto mux.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/connect", http.StatusFound)
	})
	mux.HandleFunc("GET /connect", h.connect)
	mux.HandleFunc("POST /claim", h.claim)
	mux.HandleFunc("GET /board", h.boardPage)
	mux.HandleFunc("GET /api/board", h.boardAPI)
	mux.HandleFunc("GET /stats", h.stats)
	mux.HandleFunc("GET /attend", h.attend)
	mux.HandleFunc("POST /vendor/grant", h.vendorGrant)
	mux.HandleFunc("POST /vendor/spawn_core", h.vendorSpawnCore)
	mux.HandleFunc("POST /admin/remove_agent", h.adminRemoveAgent)
}

func (h *Handler) wall(w http.ResponseWriter, r *http.Request) {
	entries := h.leaderboard.Entries()
	render(w, r, wallPage(entries))
}

func (h *Handler) connect(w http.ResponseWriter, r *http.Request) {
	render(w, r, connectPage(h.mcpURL, 5))
}

// attend serves the printable attendee quickstart. Read from disk on every
// request so organizers can update the handout by editing one file on the
// server - no rebuild, no redistribution.
func (h *Handler) attend(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/static/attend.html")
}


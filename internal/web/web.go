// Package web serves the public website: the Wall of Fame and the
// "connect your agent" page, rendered with templ. The POST /claim endpoint
// (the only write) is wired here.
package web

import (
	"net/http"

	"github.com/pisush/heyaiagents/internal/config"
	"github.com/pisush/heyaiagents/internal/content"
	"github.com/pisush/heyaiagents/internal/store"
)

// Handler holds the dependencies the website needs.
type Handler struct {
	content     *content.Store
	leaderboard *store.Leaderboard
	cfg         config.Config
	mcpURL      string
}

// NewHandler builds the website handler over the in-memory content store.
func NewHandler(c *content.Store, lb *store.Leaderboard, cfg config.Config, mcpURL string) *Handler {
	return &Handler{content: c, leaderboard: lb, cfg: cfg, mcpURL: mcpURL}
}

// Routes registers the website routes onto mux.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.wall)
	mux.HandleFunc("GET /connect", h.connect)
	mux.HandleFunc("POST /claim", h.claim)
}

func (h *Handler) wall(w http.ResponseWriter, r *http.Request) {
	entries := h.leaderboard.Entries()
	render(w, r, wallPage(entries))
}

func (h *Handler) connect(w http.ResponseWriter, r *http.Request) {
	render(w, r, connectPage(h.mcpURL, 5))
}


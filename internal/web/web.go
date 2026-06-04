// Package web serves the public website: the Wall of Fame and the
// "connect your agent" page, rendered with templ. The POST /claim endpoint
// (the only write) is wired here.
package web

import (
	"net/http"
	"strconv"

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

// rank renders a 1-based rank label from a 0-based index.
func rank(i int) string { return strconv.Itoa(i + 1) }

// sessions renders a session count with its unit.
func sessions(n int) string {
	if n == 1 {
		return "1 session"
	}
	return strconv.Itoa(n) + " sessions"
}

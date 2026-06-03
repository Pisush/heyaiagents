// Package api wires HTTP handlers for the HeyAI Agents backend using the Go
// stdlib net/http ServeMux (Go 1.22+ method-aware routing).
package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/pisush/heyaiagents/backend/internal/config"
	"github.com/pisush/heyaiagents/backend/internal/db"
)

// Server holds the dependencies shared by HTTP handlers.
type Server struct {
	DB *sql.DB
}

// NewRouter builds the application's HTTP handler.
func NewRouter(s *Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/version", s.handleVersion)
	return withCORS(mux)
}

type healthResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	resp := healthResponse{Status: "ok", DB: "ok"}
	if err := db.Ping(s.DB); err != nil {
		resp.DB = "error"
		writeJSON(w, http.StatusServiceUnavailable, resp)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

type versionResponse struct {
	Model string `json:"model"`
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, versionResponse{Model: config.ModelName})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// withCORS allows the Next.js dev server to call the API during local
// development. Tightened to specific origins before any real deployment.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

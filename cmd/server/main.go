// Command server is the single binary for the HeyAI platform. It serves the
// website (Wall of Fame + connect page) and, from Milestone 3, the read-only
// MCP surface — both from one stdlib net/http server.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pisush/heyaiagents/internal/config"
	"github.com/pisush/heyaiagents/internal/content"
	"github.com/pisush/heyaiagents/internal/store"
	"github.com/pisush/heyaiagents/internal/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	for _, w := range cfg.Warnings() {
		log.Printf("warning: %s", w)
	}

	// Leaderboard: the only durable state (a JSON file).
	leaderboard, err := store.Open(cfg.LeaderboardPath)
	if err != nil {
		log.Fatalf("leaderboard store: %v", err)
	}
	_ = leaderboard // claim/read wiring lands in Milestone 4

	// Knowledgebase: seeded into memory in Milestone 2. Empty for the scaffold.
	kb := content.New(nil, nil)

	mux := http.NewServeMux()

	// Static assets (Tailwind output + htmx).
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Website.
	site := web.NewHandler(kb, "http://localhost:"+cfg.Port+"/mcp")
	site.Routes(mux)

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (event %s → %s)", cfg.Addr(),
			cfg.EventStart.Format(time.RFC3339), cfg.EventEnd.Format(time.RFC3339))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

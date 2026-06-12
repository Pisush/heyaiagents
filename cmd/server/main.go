// Command server is the single binary for the HeyAI platform. It serves the
// website (Wall of Fame + connect page) and the read-only MCP surface — both
// from one stdlib net/http server.
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

	"github.com/pisush/heyaiagents/internal/board"
	"github.com/pisush/heyaiagents/internal/config"
	"github.com/pisush/heyaiagents/internal/content"
	mcpserver "github.com/pisush/heyaiagents/internal/mcp"
	"github.com/pisush/heyaiagents/internal/store"
	"github.com/pisush/heyaiagents/internal/vendors"
	"github.com/pisush/heyaiagents/internal/web"
	"github.com/pisush/heyaiagents/seed"
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

	// Knowledgebase: seed from embedded JSON files.
	sessions, speakers, err := seed.Load()
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Printf("loaded %d sessions, %d speakers from seed", len(sessions), len(speakers))
	kb := content.New(sessions, speakers)

	// Agent Pixels: the shared canvas (a JSON file, like the leaderboard).
	pixels, err := board.Open(cfg.BoardPath)
	if err != nil {
		log.Fatalf("board store: %v", err)
	}

	// Booth vendors (optional registry file).
	booths, err := vendors.Load(cfg.VendorsPath)
	if err != nil {
		log.Fatalf("vendors: %v", err)
	}
	log.Printf("loaded %d booth vendors", len(booths.All()))

	// Neutral data cores spawn on a timer (CORE_INTERVAL, e.g. "25m"; "off" disables).
	coreInterval := 25 * time.Minute
	if v := os.Getenv("CORE_INTERVAL"); v != "" {
		if v == "off" {
			coreInterval = 0
		} else if d, err := time.ParseDuration(v); err == nil {
			coreInterval = d
		} else {
			log.Printf("warning: bad CORE_INTERVAL %q, using %s", v, coreInterval)
		}
	}
	pixels.StartCoreSpawner(coreInterval)

	mcpURL := os.Getenv("MCP_PUBLIC_URL")
	if mcpURL == "" {
		mcpURL = "http://localhost:" + cfg.Port + "/mcp"
	}

	mux := http.NewServeMux()

	// Static assets (Tailwind output + htmx).
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// MCP server: read-only knowledgebase + Agent Pixels moves.
	mcpHandler := mcpserver.NewHandler(mcpserver.Dependencies{
		Content:     kb,
		Leaderboard: leaderboard,
		Board:       pixels,
		Vendors:     booths,
		Cfg:         cfg,
		Secret:      cfg.ServerSecret,
	})
	mux.Handle("/mcp", mcpHandler)
	mux.Handle("/mcp/", mcpHandler)

	// Website.
	site := web.NewHandler(kb, leaderboard, pixels, booths, cfg, mcpURL)
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

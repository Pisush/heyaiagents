// Command server is the HTTP entrypoint for the HeyAI Agents backend.
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

	"github.com/pisush/heyaiagents/backend/internal/api"
	"github.com/pisush/heyaiagents/backend/internal/config"
	"github.com/pisush/heyaiagents/backend/internal/db"
	"github.com/pisush/heyaiagents/backend/internal/llm"
)

func main() {
	cfg := config.Load()
	for _, warning := range cfg.Warnings() {
		log.Printf("warning: %s", warning)
	}

	conn, err := db.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer conn.Close()

	// Constructed now so misconfiguration surfaces at boot; not used until the
	// agent milestones add LLM-backed endpoints.
	_ = llm.New(cfg.AnthropicAPIKey)

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           api.NewRouter(&api.Server{DB: conn}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (model=%s, db=%s)", cfg.Addr(), config.ModelName, cfg.SQLitePath)
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

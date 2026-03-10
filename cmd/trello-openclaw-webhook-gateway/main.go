package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lonegunmanb/trello-openclaw-webhook-gateway/internal/app"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	cfg, err := app.LoadConfig(os.Args)
	if err != nil {
		logger.Fatalf("invalid config: %v", err)
	}
	logger.Printf("starting gateway %s", cfg.Redacted())

	httpClient := &http.Client{Timeout: 30 * time.Second}
	router := app.NewRouter(cfg, httpClient, logger)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	logger.Printf("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
	}
	logger.Printf("server stopped")
}

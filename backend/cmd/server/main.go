// Package main starts the AgentPulse backend server.
//
// Entry point responsibilities:
//   - load config
//   - init logger
//   - assemble dependencies (ClickHouse/PostgreSQL/Chroma)
//   - start API + OTLP HTTP servers
//   - graceful shutdown on SIGINT/SIGTERM
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentpulse/backend/internal/app"
	"github.com/agentpulse/backend/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := app.LoadConfig(".")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := logger.New(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Infof("AgentPulse starting... env=%s version=0.1.0", cfg.Server.Mode)

	application, err := app.New(cfg, log)
	if err != nil {
		return fmt.Errorf("init app: %w", err)
	}

	// serveCtx is cancelled on SIGINT/SIGTERM, which unblocks application.Serve.
	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- application.Serve(serveCtx)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		return fmt.Errorf("server failed: %w", err)
	case sig := <-quit:
		log.Infof("received signal %v, shutting down...", sig)
		cancelServe() // unblock Serve
	}

	// Graceful shutdown with the configured timeout.
	shutdownTimeout := cfg.Server.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 30 * time.Second
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		log.Errorf("shutdown error: %v", err)
		return err
	}

	log.Infof("AgentPulse stopped gracefully")
	return nil
}

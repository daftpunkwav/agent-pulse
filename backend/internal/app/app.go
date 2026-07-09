// Package app assembles AgentPulse dependencies and manages lifecycle.
//
// Design:
//   - Single entry: main.go calls LoadConfig + New + Serve + Shutdown
//   - Explicit injection: all dependencies passed as args, no globals
//   - Interface decoupling: business layer depends on Repository/Service
//     interfaces only
//   - Graceful shutdown: reverse order of dependency creation
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/agentpulse/backend/internal/api"
	"github.com/agentpulse/backend/internal/collector"
	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/repository"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// Application is the AgentPulse app container.
type Application struct {
	cfg *config.Config
	log logger.Logger

	clickhouse *repository.ClickHouseClient
	postgres   *repository.PostgresClient
	chroma     *repository.ChromaClient

	services *service.Container

	apiServer  *http.Server
	otlpServer *http.Server

	shutdownFns []ShutdownFunc
	mu          sync.Mutex
	closed      bool
}

// ShutdownFunc is a graceful-shutdown hook.
type ShutdownFunc func(context.Context) error

// LoadConfig is a thin wrapper around config.Load.
func LoadConfig(dir string) (*config.Config, error) {
	return config.Load(dir)
}

// New creates and initializes the application.
//
// If any step fails, all previously-initialized resources are torn down
// before returning the error (fail-fast with rollback).
func New(cfg *config.Config, log logger.Logger) (*Application, error) {
	app := &Application{
		cfg: cfg,
		log: log.WithFields(map[string]any{"component": "app"}),
	}

	if err := app.initInfrastructure(); err != nil {
		// Rollback any partial initialization
		_ = app.Shutdown(context.Background())
		return nil, fmt.Errorf("init infrastructure: %w", err)
	}

	repos := app.initRepositories()
	app.services = service.NewContainer(repos, cfg, log)
	// Inject self as the health pinger for /readyz.
	app.services.HealthPinger = app

	if err := app.initHTTPServers(); err != nil {
		_ = app.Shutdown(context.Background())
		return nil, fmt.Errorf("init http servers: %w", err)
	}

	return app, nil
}

func (a *Application) initInfrastructure() error {
	chClient, err := repository.NewClickHouseClient(a.cfg.ClickHouse, a.log)
	if err != nil {
		return fmt.Errorf("connect clickhouse: %w", err)
	}
	a.clickhouse = chClient
	a.onShutdown(func(ctx context.Context) error {
		a.log.Infof("closing clickhouse connection")
		return chClient.Close()
	})

	pgClient, err := repository.NewPostgresClient(a.cfg.Postgres, a.log)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	a.postgres = pgClient
	a.onShutdown(func(ctx context.Context) error {
		a.log.Infof("closing postgres connection")
		return pgClient.Close()
	})

	if a.cfg.Chroma.Host != "" {
		chromaClient, err := repository.NewChromaClient(a.cfg.Chroma, a.log)
		if err != nil {
			a.log.Warnf("connect chroma failed (non-fatal): %v", err)
		} else {
			a.chroma = chromaClient
			a.onShutdown(func(ctx context.Context) error {
				a.log.Infof("closing chroma connection")
				return chromaClient.Close()
			})
		}
	}

	return nil
}

func (a *Application) initRepositories() *repository.Container {
	clickhouseRepo := repository.NewClickHouseSpanRepo(a.clickhouse, a.log)
	postgresRepo := repository.NewPostgresMetadataRepo(a.postgres, a.log)
	evalRepo := repository.NewPostgresEvaluationRepo(a.postgres, a.log)
	pricingRepo := repository.NewPostgresPricingRepo(a.postgres, a.log)

	var vectorRepo domain.VectorRepository
	if a.chroma != nil {
		vectorRepo = repository.NewChromaVectorRepo(a.chroma, a.log)
	}

	return repository.NewContainer(repository.ContainerDeps{
		Span:       clickhouseRepo,
		Metadata:   postgresRepo,
		Evaluation: evalRepo,
		Pricing:    pricingRepo,
		Vector:     vectorRepo,
	})
}

func (a *Application) initHTTPServers() error {
	gin.SetMode(a.cfg.Server.Mode)
	router := api.NewRouter(a.cfg, a.services, a.log)
	a.apiServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", a.cfg.Server.Host, a.cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  a.cfg.Server.ReadTimeout,
		WriteTimeout: a.cfg.Server.WriteTimeout,
	}

	otlpHandler := collector.NewHTTPHandler(a.cfg, a.services, a.log)
	a.otlpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", a.cfg.OTLP.HTTPPort),
		Handler:           otlpHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return nil
}

// Serve starts all HTTP servers and blocks until either server errors or
// ctx is cancelled.
func (a *Application) Serve(ctx context.Context) error {
	errCh := make(chan error, 2)

	go func() {
		a.log.Infof("API server listening on %s", a.apiServer.Addr)
		if err := a.apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("api server: %w", err)
		}
	}()

	go func() {
		a.log.Infof("OTLP HTTP receiver listening on %s", a.otlpServer.Addr)
		if err := a.otlpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("otlp server: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}

// Shutdown gracefully releases all resources. Idempotent.
func (a *Application) Shutdown(ctx context.Context) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	a.mu.Unlock()

	a.log.Infof("shutting down AgentPulse...")

	if err := a.shutdownHTTP(ctx); err != nil {
		a.log.Errorf("shutdown http: %v", err)
	}

	if a.services != nil {
		a.services.Shutdown(ctx)
	}

	var firstErr error
	for i := len(a.shutdownFns) - 1; i >= 0; i-- {
		if err := a.shutdownFns[i](ctx); err != nil {
			a.log.Errorf("shutdown hook %d: %v", i, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

func (a *Application) shutdownHTTP(ctx context.Context) error {
	var firstErr error

	if a.apiServer != nil {
		if err := a.apiServer.Shutdown(ctx); err != nil {
			firstErr = fmt.Errorf("api shutdown: %w", err)
		}
	}

	if a.otlpServer != nil {
		if err := a.otlpServer.Shutdown(ctx); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("otlp shutdown: %w", err)
			}
		}
	}

	return firstErr
}

func (a *Application) onShutdown(fn ShutdownFunc) {
	a.mu.Lock()
	a.shutdownFns = append(a.shutdownFns, fn)
	a.mu.Unlock()
}

// ListenerAddr returns the API server's bind address.
func (a *Application) ListenerAddr() string {
	if a.apiServer == nil {
		return ""
	}
	return a.apiServer.Addr
}

// NetListener is a helper for tests to inject a custom listener.
func NetListener(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

// HealthCheck performs readiness probes against all critical dependencies.
// Returns the first failing dependency's name + error.
func (a *Application) HealthCheck() error {
	type check struct {
		name string
		fn   func() error
	}
	checks := []check{
		{"clickhouse", a.clickhouse.Ping},
		{"postgres", a.postgres.Ping},
	}
	if a.chroma != nil {
		checks = append(checks, check{"chroma", a.chroma.Ping})
	}

	for _, c := range checks {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = ctx
		if err := c.fn(); err != nil {
			cancel()
			return fmt.Errorf("%s: %w", c.name, err)
		}
		cancel()
	}
	return nil
}

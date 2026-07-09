// Package app 负责 AgentPulse 应用的依赖装配与生命周期管理。
//
// 设计原则：
//   - 单一入口：main.go 仅调用 LoadConfig + New + Serve + Shutdown
//   - 显式依赖：所有依赖通过参数注入，不使用全局变量
//   - 接口解耦：业务层只依赖 Repository/Service 接口
//   - 优雅关闭：按依赖顺序反向关闭资源
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

// Application 是 AgentPulse 的应用容器。
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

// ShutdownFunc 是优雅关闭钩子。
type ShutdownFunc func(context.Context) error

// LoadConfig 是 config.Load 的薄包装。
func LoadConfig(dir string) (*config.Config, error) {
	return config.Load(dir)
}

// New 创建并初始化应用。
func New(cfg *config.Config, log logger.Logger) (*Application, error) {
	app := &Application{
		cfg: cfg,
		log: log.WithFields(map[string]any{"component": "app"}),
	}

	if err := app.initInfrastructure(); err != nil {
		return nil, fmt.Errorf("init infrastructure: %w", err)
	}

	repos := app.initRepositories()
	app.services = service.NewContainer(repos, log)

	if err := app.initHTTPServers(); err != nil {
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

// Serve 启动所有 HTTP 服务。
func (a *Application) Serve() error {
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
	case <-a.context().Done():
		return nil
	}
}

// Shutdown 优雅关闭所有资源。
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

func (a *Application) context() context.Context {
	return context.Background()
}

func (a *Application) onShutdown(fn ShutdownFunc) {
	a.mu.Lock()
	a.shutdownFns = append(a.shutdownFns, fn)
	a.mu.Unlock()
}

// ListenerAddr 返回主 API 服务实际监听地址。
func (a *Application) ListenerAddr() string {
	if a.apiServer == nil {
		return ""
	}
	return a.apiServer.Addr
}

// NetListener 辅助测试注入自定义 listener。
func NetListener(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

// HealthCheck 用于健康检查端点。
func (a *Application) HealthCheck() error {
	checks := []struct {
		name string
		fn   func() error
	}{
		{"clickhouse", a.clickhouse.Ping},
		{"postgres", a.postgres.Ping},
	}
	if a.chroma != nil {
		checks = append(checks, struct {
			name string
			fn   func() error
		}{"chroma", a.chroma.Ping})
	}

	for _, c := range checks {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = ctx
		defer cancel()
		if err := c.fn(); err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
	}
	return nil
}
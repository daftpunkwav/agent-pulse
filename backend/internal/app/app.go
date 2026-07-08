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
//
// 持有所有运行时依赖：HTTP 服务、OTLP 接收器、基础设施连接。
// 通过 Serve/Shutdown 控制生命周期。
type Application struct {
	cfg *config.Config
	log logger.Logger

	// 基础设施连接
	clickhouse *repository.ClickHouseClient
	postgres   *repository.PostgresClient
	chroma     *repository.ChromaClient

	// 业务服务
	services *service.Container

	// HTTP 服务
	apiServer  *http.Server
	otlpServer *http.Server

	// 关闭钩子
	shutdownFns []ShutdownFunc
	mu          sync.Mutex
	closed      bool
}

// ShutdownFunc 是优雅关闭钩子。
//
// 按注册顺序的逆序执行。
type ShutdownFunc func(context.Context) error

// LoadConfig 是 config.Load 的薄包装，便于 main.go 调用。
func LoadConfig(dir string) (*config.Config, error) {
	return config.Load(dir)
}

// New 创建并初始化应用。
//
// 初始化顺序：
//   1. 基础设施连接（ClickHouse/PG/Chroma）
//   2. Repository 层
//   3. Service 层
//   4. HTTP 路由与中间件
func New(cfg *config.Config, log logger.Logger) (*Application, error) {
	app := &Application{
		cfg: cfg,
		log: log.WithFields(map[string]any{
			"component": "app",
		}),
	}

	// 1. 初始化基础设施
	if err := app.initInfrastructure(); err != nil {
		return nil, fmt.Errorf("init infrastructure: %w", err)
	}

	// 2. 初始化 Repository 层
	repos := app.initRepositories()

	// 3. 初始化 Service 层
	app.services = service.NewContainer(repos, log)

	// 4. 初始化 HTTP 服务
	if err := app.initHTTPServers(); err != nil {
		return nil, fmt.Errorf("init http servers: %w", err)
	}

	return app, nil
}

// ---------------------------------------------------------------------------
// 初始化
// ---------------------------------------------------------------------------

func (a *Application) initInfrastructure() error {
	// ClickHouse
	chClient, err := repository.NewClickHouseClient(a.cfg.ClickHouse, a.log)
	if err != nil {
		return fmt.Errorf("connect clickhouse: %w", err)
	}
	a.clickhouse = chClient
	a.onShutdown(func(ctx context.Context) error {
		a.log.Info("closing clickhouse connection")
		return chClient.Close()
	})

	// PostgreSQL
	pgClient, err := repository.NewPostgresClient(a.cfg.Postgres, a.log)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	a.postgres = pgClient
	a.onShutdown(func(ctx context.Context) error {
		a.log.Info("closing postgres connection")
		return pgClient.Close()
	})

	// Chroma（可选：连接失败不阻塞启动）
	if a.cfg.Chroma.Host != "" {
		chromaClient, err := repository.NewChromaClient(a.cfg.Chroma, a.log)
		if err != nil {
			a.log.Warnf("connect chroma failed (non-fatal): %v", err)
		} else {
			a.chroma = chromaClient
			a.onShutdown(func(ctx context.Context) error {
				a.log.Info("closing chroma connection")
				return chromaClient.Close()
			})
		}
	}

	return nil
}

func (a *Application) initRepositories() *repository.Container {
	clickhouseRepo := repository.NewClickHouseSpanRepository(a.clickhouse, a.log)
	postgresRepo := repository.NewPostgresMetadataRepository(a.postgres, a.log)
	evalRepo := repository.NewPostgresEvaluationRepository(a.postgres, a.log)
	pricingRepo := repository.NewPostgresPricingRepository(a.postgres, a.log)

	var vectorRepo domain.VectorRepository
	if a.chroma != nil {
		vectorRepo = repository.NewChromaVectorRepository(a.chroma, a.log)
	}

	return repository.NewContainer(repository.ContainerDeps{
		Span:        clickhouseRepo,
		Metadata:    postgresRepo,
		Evaluation:  evalRepo,
		Pricing:     pricingRepo,
		Vector:      vectorRepo,
	})
}

func (a *Application) initHTTPServers() error {
	// 主 API 服务
	gin.SetMode(a.cfg.Server.Mode)
	router := api.NewRouter(a.cfg, a.services, a.log)
	a.apiServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", a.cfg.Server.Host, a.cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  a.cfg.Server.ReadTimeout,
		WriteTimeout: a.cfg.Server.WriteTimeout,
	}

	// OTLP HTTP 接收器
	otlpHandler := collector.NewHTTPHandler(a.services, a.log)
	a.otlpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", a.cfg.OTLP.HTTPPort),
		Handler: otlpHandler,
	}

	return nil
}

// ---------------------------------------------------------------------------
// 生命周期
// ---------------------------------------------------------------------------

// Serve 启动所有 HTTP 服务并阻塞等待。
//
// 任一服务异常返回时，整体退出。
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

	a.log.Info("shutting down AgentPulse...")

	// 1. 停止接受新请求
	if err := a.shutdownHTTP(ctx); err != nil {
		a.log.Errorf("shutdown http: %v", err)
	}

	// 2. 关闭服务层（清理 goroutine）
	if a.services != nil {
		a.services.Shutdown(ctx)
	}

	// 3. 执行注册的关闭钩子（逆序）
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

// ListenerAddr 返回主 API 服务实际监听地址（用于测试）。
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
		defer cancel()
		_ = ctx
		if err := c.fn(); err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
	}
	return nil
}
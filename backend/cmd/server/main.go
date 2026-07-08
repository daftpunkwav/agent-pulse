// Package main 启动 AgentPulse 后端服务。
//
// AgentPulse 是面向 LLM Agent 的运维平台，
// 提供 Trace 采集、成本归因、在线评估、失败聚类等能力。
//
// 本入口负责：
//   - 加载配置
//   - 初始化基础设施连接（ClickHouse/PostgreSQL/Chroma）
//   - 装配依赖注入容器
//   - 启动 HTTP 服务（API + OTLP 接收）
//   - 优雅关闭
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
	// 加载配置
	cfg, err := app.LoadConfig(".")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 初始化日志
	log, err := logger.New(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Infof("AgentPulse starting... env=%s version=0.1.0", cfg.Server.Mode)

	// 装配应用
	application, err := app.New(cfg, log)
	if err != nil {
		return fmt.Errorf("init app: %w", err)
	}

	// 启动服务（异步）
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- application.Serve()
	}()

	// 等待信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		return fmt.Errorf("server failed: %w", err)
	case sig := <-quit:
		log.Infof("received signal %v, shutting down...", sig)
	}

	// 优雅关闭（超时 30s）
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		log.Errorf("shutdown error: %v", err)
		return err
	}

	log.Infof("AgentPulse stopped gracefully")
	return nil
}
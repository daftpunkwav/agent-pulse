// Package repository - PostgreSQL 客户端封装。
//
// 使用 pgx/v5 官方驱动，性能优于 database/sql。
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/pkg/logger"
)

// PostgresClient 封装 PostgreSQL 连接池。
type PostgresClient struct {
	pool   *pgxpool.Pool
	logger logger.Logger
}

// NewPostgresClient 创建 PostgreSQL 客户端。
func NewPostgresClient(cfg config.PostgresConfig, log logger.Logger) (*PostgresClient, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse pg config: %w", err)
	}

	// 池配置
	if cfg.MaxOpenConns > 0 {
		poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		poolCfg.MinConns = int32(cfg.MaxIdleConns)
	}
	if cfg.MaxLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxLifetime
	}
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pg pool: %w", err)
	}

	// 启动时验证
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	log.Infof("connected to postgres at %s:%d db=%s",
		cfg.Host, cfg.Port, cfg.Database)

	return &PostgresClient{
		pool:   pool,
		logger: log.WithFields(map[string]any{"component": "postgres"}),
	}, nil
}

// Pool 返回原生 pgxpool.Pool。
//
// 仓储层使用此 Pool 执行查询。
func (c *PostgresClient) Pool() *pgxpool.Pool {
	return c.pool
}

// Ping 健康检查。
func (c *PostgresClient) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return c.pool.Ping(ctx)
}

// Close 关闭连接池。
func (c *PostgresClient) Close() error {
	c.pool.Close()
	return nil
}
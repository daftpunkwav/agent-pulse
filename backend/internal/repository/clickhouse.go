// Package repository - ClickHouse 客户端封装。
//
// 职责：
//   - 管理 ClickHouse 连接池
//   - 提供 Ping/Close 生命周期方法
//   - 提供原生 *clickhouse.Conn 给仓储层使用
//
// 使用 clickhouse-go/v2 官方驱动，支持 ClickHouse 24+。
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/pkg/logger"
)

// ClickHouseClient 封装 ClickHouse 连接。
type ClickHouseClient struct {
	conn   driver.Conn
	logger logger.Logger
}

// NewClickHouseClient 创建 ClickHouse 客户端。
//
// 使用 OpenDB + 连接池模式，便于管理大量短查询。
func NewClickHouseClient(cfg config.ClickHouseConfig, log logger.Logger) (*ClickHouseClient, error) {
	opts := &clickhouse.Options{
		Addr:         []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth:         clickhouse.Auth{Database: cfg.Database, Username: cfg.Username, Password: cfg.Password},
		MaxOpenConns: cfg.MaxOpenConns,
		MaxIdleConns: cfg.MaxIdleConns,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		// 压缩降低网络开销
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		// 调试
		Debug: false,
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}

	// 启动时验证连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}

	log.Infof("connected to clickhouse at %s:%d db=%s", cfg.Host, cfg.Port, cfg.Database)

	return &ClickHouseClient{
		conn:   conn,
		logger: log.WithFields(map[string]any{"component": "clickhouse"}),
	}, nil
}

// Conn 返回原生 driver.Conn。
//
// 仓储层使用此 Conn 执行原生 SQL。
func (c *ClickHouseClient) Conn() driver.Conn {
	return c.conn
}

// Ping 健康检查。
func (c *ClickHouseClient) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return c.conn.Ping(ctx)
}

// Close 关闭连接。
func (c *ClickHouseClient) Close() error {
	return c.conn.Close()
}

// Exec 执行无返回结果的 SQL（如 DDL）。
func (c *ClickHouseClient) Exec(ctx context.Context, query string, args ...any) error {
	return c.conn.Exec(ctx, query, args...)
}

// QueryRow 查询单行。
func (c *ClickHouseClient) QueryRow(ctx context.Context, dest any, query string, args ...any) error {
	row := c.conn.QueryRow(ctx, query, args...)
	return row.ScanStruct(dest)
}

// QueryRows 查询多行。
//
// 使用 scanFn 处理每行，回调返回 false 停止迭代。
func (c *ClickHouseClient) QueryRows(ctx context.Context, query string, scanFn func(rows driver.Rows) error, args ...any) error {
	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		if err := scanFn(rows); err != nil {
			return err
		}
	}

	return rows.Err()
}

// AsyncInsert 异步批量插入（ClickHouse 推荐方式）。
//
// 适合高吞吐写入场景，数据先入 buffer，ClickHouse 自动刷盘。
func (c *ClickHouseClient) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return c.conn.AsyncInsert(ctx, query, wait, args...)
}
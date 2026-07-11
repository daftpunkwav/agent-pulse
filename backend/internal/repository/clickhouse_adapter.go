// Package repository - ClickHouse 仓储层适配。
//
// 本文件将 repository.ClickHouseClient 适配为 domain.ClickHouseQueryExecutor 接口，
// 使业务层（CostService 等）不依赖 clickhouse-go 驱动。
package repository

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/agentpulse/backend/internal/domain"
)

// Assert ClickHouseQueryExecutorAdapter implements domain.ClickHouseQueryExecutor at compile time.
var _ domain.ClickHouseQueryExecutor = (*ClickHouseQueryExecutorAdapter)(nil)

// ClickHouseQueryExecutorAdapter 将 ClickHouseClient 适配为 domain.ClickHouseQueryExecutor。
type ClickHouseQueryExecutorAdapter struct {
	client *ClickHouseClient
}

// NewClickHouseQueryExecutorAdapter 创建适配器。
func NewClickHouseQueryExecutorAdapter(client *ClickHouseClient) *ClickHouseQueryExecutorAdapter {
	return &ClickHouseQueryExecutorAdapter{client: client}
}

// QueryRows 实现 domain.ClickHouseQueryExecutor。
func (a *ClickHouseQueryExecutorAdapter) QueryRows(
	ctx context.Context,
	query string,
	scanFn func(rows domain.Rows) error,
	args ...any,
) error {
	return a.client.QueryRows(ctx, query, func(rows driver.Rows) error {
		return scanFn(&rowsAdapter{rows: rows})
	}, args...)
}

// QueryRow 实现 domain.ClickHouseQueryExecutor。
func (a *ClickHouseQueryExecutorAdapter) QueryRow(
	ctx context.Context,
	query string,
	scanFn func(row domain.Row) error,
	args ...any,
) error {
	row := a.client.Conn().QueryRow(ctx, query, args...)
	return scanFn(&rowAdapter{row: row})
}

// Conn 实现 domain.ClickHouseQueryExecutor。
func (a *ClickHouseQueryExecutorAdapter) Conn() domain.Conn {
	return &connAdapter{conn: a.client.Conn()}
}

// ---------------------------------------------------------------------------
// 适配器：将 clickhouse-go driver 类型包装为 domain 接口
// ---------------------------------------------------------------------------

// rowsAdapter 包装 driver.Rows 实现 domain.Rows。
type rowsAdapter struct {
	rows driver.Rows
}

func (r *rowsAdapter) Next() bool {
	return r.rows.Next()
}

func (r *rowsAdapter) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

func (r *rowsAdapter) Err() error {
	return r.rows.Err()
}

func (r *rowsAdapter) Close() error {
	return r.rows.Close()
}

// rowAdapter 包装 driver.Row 实现 domain.Row。
type rowAdapter struct {
	row driver.Row
}

func (r *rowAdapter) Scan(dest ...any) error {
	return r.row.Scan(dest...)
}

// connAdapter 包装 driver.Conn 实现 domain.Conn。
type connAdapter struct {
	conn driver.Conn
}

func (c *connAdapter) Exec(ctx context.Context, query string, args ...any) error {
	return c.conn.Exec(ctx, query, args...)
}

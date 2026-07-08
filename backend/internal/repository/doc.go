// Package repository 提供基础设施连接与仓储实现。
//
// 子包划分：
//   - clickhouse.go: ClickHouse 连接管理
//   - postgres.go: PostgreSQL 连接管理
//   - chroma.go: Chroma 向量库连接管理
//   - container.go: 仓储容器（聚合所有 Repository 实例）
//   - span_*.go: Span 仓储实现（按存储后端拆分）
//   - evaluation_*.go: Evaluation 仓储实现
//   - pricing_*.go: Pricing 仓储实现
//   - metadata_*.go: Metadata 仓储实现（Harness/AB Test/Cluster）
//   - vector_*.go: 向量仓储实现
package repository

import (
	"context"

	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/pkg/logger"
)

// RepositoryContainerOptions 仓储容器配置。
type RepositoryContainerOptions struct {
	Config *config.Config
	Logger logger.Logger
}

// ContextKey 用于在 context 中传递特定值（如 trace_id）。
type ContextKey string

const (
	// ContextKeyTraceID 用于在 context 中传递当前 trace ID。
	ContextKeyTraceID ContextKey = "trace_id"
	// ContextKeySessionID 用于在 context 中传递当前 session ID。
	ContextKeySessionID ContextKey = "session_id"
	// ContextKeyUserID 用于在 context 中传递当前 user ID。
	ContextKeyUserID ContextKey = "user_id"
)

// GetContextValue 安全地从 context 读取值。
func GetContextValue(ctx context.Context, key ContextKey) string {
	if v, ok := ctx.Value(key).(string); ok {
		return v
	}
	return ""
}
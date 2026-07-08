// Package repository - 仓储容器。
//
// 聚合所有 Repository 实例，注入到 Service 层。
// 使用 ContainerDeps 显式声明依赖，便于测试替换。
package repository

import (
	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/pkg/logger"
)

// Container 仓储容器。
//
// 持有所有 Repository 实例，Service 层通过此容器访问。
type Container struct {
	Span       domain.SpanRepository
	Evaluation domain.EvaluationRepository
	Pricing    domain.PricingRepository
	Metadata   domain.MetadataRepository
	Vector     domain.VectorRepository // 可选
}

// ContainerDeps 仓储依赖。
//
// 显式声明，便于测试时只注入需要的 mock。
type ContainerDeps struct {
	Span       domain.SpanRepository
	Evaluation domain.EvaluationRepository
	Pricing    domain.PricingRepository
	Metadata   domain.MetadataRepository
	Vector     domain.VectorRepository
}

// NewContainer 创建仓储容器。
func NewContainer(deps ContainerDeps) *Container {
	return &Container{
		Span:       deps.Span,
		Evaluation: deps.Evaluation,
		Pricing:    deps.Pricing,
		Metadata:   deps.Metadata,
		Vector:     deps.Vector,
	}
}

// ---------------------------------------------------------------------------
// 工厂方法（提供默认实现）
// ---------------------------------------------------------------------------

// NewClickHouseSpanRepository 创建 ClickHouse Span 仓储。
func NewClickHouseSpanRepository(client *ClickHouseClient, log logger.Logger) domain.SpanRepository {
	return NewClickHouseSpanRepo(client, log)
}

// NewPostgresMetadataRepository 创建 PostgreSQL Metadata 仓储。
func NewPostgresMetadataRepository(client *PostgresClient, log logger.Logger) domain.MetadataRepository {
	return NewPostgresMetadataRepo(client, log)
}

// NewPostgresEvaluationRepository 创建 PostgreSQL Evaluation 仓储。
func NewPostgresEvaluationRepository(client *PostgresClient, log logger.Logger) domain.EvaluationRepository {
	return NewPostgresEvaluationRepo(client, log)
}

// NewPostgresPricingRepository 创建 PostgreSQL Pricing 仓储。
func NewPostgresPricingRepository(client *PostgresClient, log logger.Logger) domain.PricingRepository {
	return NewPostgresPricingRepo(client, log)
}

// NewChromaVectorRepository 创建 Chroma 向量仓储。
func NewChromaVectorRepository(client *ChromaClient, log logger.Logger) domain.VectorRepository {
	return NewChromaVectorRepo(client, log)
}
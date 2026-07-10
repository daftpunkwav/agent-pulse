// Package repository - 仓储容器。
//
// 聚合所有 Repository 实例，注入到 Service 层。
// 使用 ContainerDeps 显式声明依赖，便于测试替换。
package repository

import (
	"github.com/agentpulse/backend/internal/domain"
)

// Container 仓储容器。
//
// 持有所有 Repository 实例，Service 层通过此容器访问。
type Container struct {
	Span             domain.SpanRepository
	Evaluation       domain.EvaluationRepository
	Pricing          domain.PricingRepository
	Metadata         domain.MetadataRepository
	Vector           domain.VectorRepository // 可选
	ClickHouseExecutor domain.ClickHouseQueryExecutor // ClickHouse 查询执行器（CostService 使用）
}

// ContainerDeps 仓储依赖。
//
// 显式声明，便于测试时只注入需要的 mock。
type ContainerDeps struct {
	Span             domain.SpanRepository
	Evaluation       domain.EvaluationRepository
	Pricing          domain.PricingRepository
	Metadata         domain.MetadataRepository
	Vector           domain.VectorRepository
	ClickHouseExecutor domain.ClickHouseQueryExecutor
}

// NewContainer 创建仓储容器。
func NewContainer(deps ContainerDeps) *Container {
	return &Container{
		Span:             deps.Span,
		Evaluation:       deps.Evaluation,
		Pricing:          deps.Pricing,
		Metadata:         deps.Metadata,
		Vector:           deps.Vector,
		ClickHouseExecutor: deps.ClickHouseExecutor,
	}
}
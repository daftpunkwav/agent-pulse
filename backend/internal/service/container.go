// Package service 提供 AgentPulse 的业务服务。
//
// 服务分层：
//   - container.go:  ServiceContainer 聚合所有服务实例
//   - span_service.go:     Span 写入与查询服务
//   - cost_service.go:     五维成本归因服务
//   - eval_service.go:     在线评估服务（LLM-as-Judge）
//   - cluster_service.go:  失败聚类服务
//
// 所有服务通过接口暴露，便于测试替换实现。
package service

import (
	"context"

	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/repository"
	"github.com/agentpulse/backend/pkg/logger"
)

// Container 是所有服务的聚合根。
//
// Service 层依赖注入容器，handler 通过此容器访问服务。
type Container struct {
	SpanRepo      domain.SpanRepository
	EvalRepo      domain.EvaluationRepository
	PricingRepo   domain.PricingRepository
	MetadataRepo  domain.MetadataRepository
	VectorRepo    domain.VectorRepository

	// 业务服务
	SpanService     *SpanService
	CostService     *CostService
	EvalService     *EvalService
	ClusterService  *ClusterService
	Judge           domain.Judge

	// HealthPinger is injected by Application for /readyz.
	// Optional: nil means readiness returns 503.
	HealthPinger interface {
		HealthCheck() error
	}
}

// NewContainer 创建服务容器。
func NewContainer(repos *repository.Container, cfg *config.Config, log logger.Logger) *Container {
	c := &Container{
		SpanRepo:     repos.Span,
		EvalRepo:     repos.Evaluation,
		PricingRepo:  repos.Pricing,
		MetadataRepo: repos.Metadata,
		VectorRepo:   repos.Vector,
	}

	c.SpanService = NewSpanService(repos.Span, repos.Pricing, log)
	c.CostService = NewCostService(repos.ClickHouseExecutor, repos.Span, repos.Pricing, log)

	evalCfg := EvalServiceConfig{
		Model:      cfg.Judge.Model,
		APIKey:     cfg.Judge.APIKey,
		BaseURL:    cfg.Judge.BaseURL,
		Timeout:    cfg.Judge.Timeout,
		SampleRate: float32(cfg.Evaluation.SampleRate),
		Workers:    cfg.Evaluation.AsyncWorkers,
		QueueSize:  cfg.Evaluation.AsyncQueueSize,
	}
	c.EvalService = NewEvalService(repos.Evaluation, repos.Span, evalCfg, log)
	c.ClusterService = NewClusterService(repos.Span, repos.Metadata, repos.Vector, log)

	return c
}

// Shutdown 优雅关闭所有服务。
func (c *Container) Shutdown(ctx context.Context) {
	// 关闭 LLM Judge 客户端
	if c.EvalService != nil {
		c.EvalService.Shutdown(ctx)
	}
}

// IngestSpans 实现 collector.ServiceContainer 接口。
func (c *Container) IngestSpans(ctx context.Context, spans []*domain.Span) error {
	if c.SpanService == nil {
		return nil
	}
	return c.SpanService.IngestSpans(ctx, spans)
}


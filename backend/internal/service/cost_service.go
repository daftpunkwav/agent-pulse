// Package service - 五维成本归因服务。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/repository"
	"github.com/agentpulse/backend/pkg/logger"
)

// CostService 五维成本归因服务。
//
// 五个归因维度：用户、会话、Agent、工具、推理步骤、模型
type CostService struct {
	client      *repository.ClickHouseClient
	spanRepo    domain.SpanRepository
	pricingRepo domain.PricingRepository
	logger      logger.Logger
}

// NewCostService 创建服务实例。
func NewCostService(
	spanRepo domain.SpanRepository,
	pricingRepo domain.PricingRepository,
	log logger.Logger,
) *CostService {
	s := &CostService{
		spanRepo:    spanRepo,
		pricingRepo: pricingRepo,
		logger:      log.WithFields(map[string]any{"component": "cost_service"}),
	}

	// 提取 ClickHouse 客户端（如果是 ClickHouse 后端）
	if chRepo, ok := spanRepo.(*repository.ClickHouseSpanRepo); ok {
		s.client = chRepo.Client()
	}

	return s
}

// Breakdown 按指定维度归因成本。
//
// 支持多维度同时归因，结果按 cost 降序。
func (s *CostService) Breakdown(
	ctx context.Context,
	window domain.TimeWindow,
	dimensions []domain.CostDimension,
	limit int,
) ([]*domain.CostBreakdown, error) {
	if len(dimensions) == 0 {
		dimensions = domain.AllCostDimensions()
	}
	if limit <= 0 {
		limit = 100
	}

	results := make([]*domain.CostBreakdown, 0, len(dimensions))

	for _, dim := range dimensions {
		bd, err := s.breakdownOne(ctx, dim, window, limit)
		if err != nil {
			return nil, fmt.Errorf("breakdown %s: %w", dim, err)
		}
		results = append(results, bd)
	}

	return results, nil
}

func (s *CostService) breakdownOne(
	ctx context.Context,
	dim domain.CostDimension,
	window domain.TimeWindow,
	limit int,
) (*domain.CostBreakdown, error) {
	if s.client == nil {
		return nil, fmt.Errorf("clickhouse client not available")
	}

	dimCol, whereExtra, spanType, err := dimensionConfig(dim)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS key,
			sum(toFloat64(cost_usd)) AS cost_usd,
			sum(total_tokens) AS tokens,
			count() AS call_count
		FROM agent_spans
		WHERE timestamp >= ? AND timestamp <= ?
		  AND span_type = ?
		  %s
		GROUP BY key
		ORDER BY cost_usd DESC
		LIMIT ?
	`, dimCol, whereExtra)

	bd := &domain.CostBreakdown{
		Dimension: dim,
		Window:    window,
		Items:     make([]domain.CostBreakdownItem, 0),
	}

	var totalCost float64
	var totalTokens uint64

	err = s.client.QueryRows(ctx, query, func(rows driver.Rows) error {
		var (
			key       string
			costUSD   float64
			tokens    uint64
			callCount uint64
		)
		if err := rows.Scan(&key, &costUSD, &tokens, &callCount); err != nil {
			return err
		}
		bd.Items = append(bd.Items, domain.CostBreakdownItem{
			Key:       key,
			CostUSD:   costUSD,
			Tokens:    tokens,
			CallCount: callCount,
			Rank:      len(bd.Items) + 1,
		})
		totalCost += costUSD
		totalTokens += tokens
		return nil
	}, window.From, window.To, spanType, limit)

	if err != nil {
		return nil, err
	}

	bd.TotalUSD = totalCost
	bd.TotalTokens = totalTokens
	return bd, nil
}

// TotalCost 查询时间窗口内总成本。
func (s *CostService) TotalCost(ctx context.Context, window domain.TimeWindow) (float64, uint64, error) {
	if s.client == nil {
		return 0, 0, fmt.Errorf("clickhouse client not available")
	}

	query := `
		SELECT
			sum(cost_usd) AS total_cost,
			sum(total_tokens) AS total_tokens
		FROM agent_spans
		WHERE timestamp >= ? AND timestamp <= ? AND span_type = 'llm'
	`

	var result struct {
		// 用 SQL 端 toFloat64 转换,避免 clickhouse-go 的 Decimal→float64 unsupported 错误。
		TotalCost   float64 `ch:"total_cost"`
		TotalTokens uint64  `ch:"total_tokens"`
	}

	_ = query
	row := s.client.Conn().QueryRow(ctx, strings.Replace(query, "sum(cost_usd)", "sum(toFloat64(cost_usd))", 1), window.From, window.To)
	if err := row.ScanStruct(&result); err != nil {
		return 0, 0, fmt.Errorf("query total cost: %w", err)
	}

	return result.TotalCost, result.TotalTokens, nil
}

// Timeline 返回成本时间序列。
func (s *CostService) Timeline(
	ctx context.Context,
	window domain.TimeWindow,
	granularity string,
) ([]TimelinePoint, error) {
	if s.client == nil {
		return nil, fmt.Errorf("clickhouse client not available")
	}

	var trunc string
	switch granularity {
	case "hour":
		trunc = "toStartOfHour"
	case "day":
		trunc = "toStartOfDay"
	default:
		trunc = "toStartOfHour"
	}

	query := fmt.Sprintf(`
		SELECT
			%s(timestamp) AS bucket,
			sum(toFloat64(cost_usd)) AS cost_usd,
			sum(total_tokens) AS tokens,
			count() AS call_count
		FROM agent_spans
		WHERE timestamp >= ? AND timestamp <= ? AND span_type = 'llm'
		GROUP BY bucket
		ORDER BY bucket ASC
	`, trunc)

	rows, err := s.client.Conn().Query(ctx, query, window.From, window.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []TimelinePoint
	for rows.Next() {
		var point TimelinePoint
		if err := rows.ScanStruct(&point); err != nil {
			return nil, err
		}
		points = append(points, point)
	}

	return points, rows.Err()
}

// TimelinePoint 时间序列数据点。
type TimelinePoint struct {
	Bucket    time.Time `ch:"bucket" json:"bucket"`
	CostUSD   float64   `ch:"cost_usd" json:"cost_usd"`
	Tokens    uint64    `ch:"tokens" json:"tokens"`
	CallCount uint64    `ch:"call_count" json:"call_count"`
}

// ---------------------------------------------------------------------------
// 内部辅助
// ---------------------------------------------------------------------------

// dimensionConfig 维度对应的 SQL 列名、WHERE 条件与 span_type 过滤。
func dimensionConfig(dim domain.CostDimension) (col, whereExtra, spanType string, err error) {
	switch dim {
	case domain.DimensionUser:
		return "user_id", " AND user_id != ''", "llm", nil
	case domain.DimensionSession:
		return "session_id", " AND session_id != ''", "llm", nil
	case domain.DimensionAgent:
		return "agent_name", "", "llm", nil
	case domain.DimensionTool:
		return "tool_name", " AND tool_name != ''", "tool", nil
	case domain.DimensionReasoning:
		return "toString(reasoning_step)", "", "reasoning", nil
	case domain.DimensionModel:
		return "model", " AND model != ''", "llm", nil
	default:
		return "", "", "", fmt.Errorf("unknown dimension: %s", dim)
	}
}
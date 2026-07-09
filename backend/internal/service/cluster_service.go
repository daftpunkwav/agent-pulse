// Package service - 失败聚类服务。
//
// 当前实现：基于规则 + LLM 标注的简单聚类。
//
// Phase 2 将扩展为：
//   - 向量聚类（DBSCAN）
//   - 自动 Embedding 生成
//   - 多维度失败模式分析
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/pkg/logger"
)

// ClusterService 失败聚类服务。
type ClusterService struct {
	spanRepo    domain.SpanRepository
	metadataRepo domain.MetadataRepository
	vectorRepo  domain.VectorRepository
	logger      logger.Logger
}

// NewClusterService 创建聚类服务实例。
func NewClusterService(
	spanRepo domain.SpanRepository,
	metadataRepo domain.MetadataRepository,
	vectorRepo domain.VectorRepository,
	log logger.Logger,
) *ClusterService {
	return &ClusterService{
		spanRepo:    spanRepo,
		metadataRepo: metadataRepo,
		vectorRepo:  vectorRepo,
		logger:      log.WithFields(map[string]any{"component": "cluster_service"}),
	}
}

// RunAnalysis 执行失败聚类分析。
//
// Phase 1 简化实现：
//   1. 查询时间窗口内的错误 Span
//   2. 按错误信息模式分组
//   3. 生成聚类结果
func (s *ClusterService) RunAnalysis(
	ctx context.Context,
	window domain.TimeWindow,
) ([]*domain.FailureCluster, error) {
	// 1. 查询错误 Span
	opts := domain.ListOptions{
		From:    &window.From,
		To:      &window.To,
		Status:  domain.SpanStatusError,
		Limit:   1000,
		OrderBy: "timestamp",
		OrderDesc: false,
	}

	// 获取所有 agent 的错误(全量,使用 ListAllInWindow 显式表示意图)。
	// 之前用 ListByUser(ctx, "") 实际只返回 user_id='' 的 Span,导致聚类空跑。
	spans, err := s.spanRepo.ListAllInWindow(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list error spans: %w", err)
	}

	if len(spans) == 0 {
		s.logger.Infof("no error spans in window, skip clustering")
		return nil, nil
	}

	// 2. 按错误模式分组（基于 error_message 相似度）
	clusters := s.groupByErrorPattern(spans)

	// 3. 持久化
	for _, cluster := range clusters {
		if err := s.metadataRepo.InsertFailureCluster(ctx, cluster); err != nil {
			s.logger.Errorf("insert cluster %s: %v", cluster.Name, err)
		}
	}

	s.logger.Infof("generated %d failure clusters from %d error spans", len(clusters), len(spans))
	return clusters, nil
}

// groupByErrorPattern 基于错误信息模式分组。
//
// Phase 1 简化：相同 error_message 前缀归为一组。
// Phase 2 将替换为 LLM 标注 + 向量聚类。
func (s *ClusterService) groupByErrorPattern(spans []*domain.Span) []*domain.FailureCluster {
	groups := make(map[string][]*domain.Span)

	for _, span := range spans {
		key := classifyError(span)
		groups[key] = append(groups[key], span)
	}

	clusters := make([]*domain.FailureCluster, 0, len(groups))
	totalCount := len(spans)

	for name, ss := range groups {
		cluster := &domain.FailureCluster{
			Name:          name,
			Description:   generateDescription(name, ss),
			TraceCount:    len(ss),
			Percentage:    float32(len(ss)) / float32(totalCount),
			CommonPattern: extractPattern(ss),
			Suggestion:    generateSuggestion(name),
			ExampleTraces: extractTraceIDs(ss, 5),
			IsActive:      true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		clusters = append(clusters, cluster)
	}

	return clusters
}

// GetLatestClusters 获取最近一次聚类结果。
func (s *ClusterService) GetLatestClusters(ctx context.Context) ([]*domain.FailureCluster, error) {
	return s.metadataRepo.ListFailureClusters(ctx, true)
}

// GetCluster 获取单个聚类。
func (s *ClusterService) GetCluster(ctx context.Context, id string) (*domain.FailureCluster, error) {
	clusters, err := s.metadataRepo.ListFailureClusters(ctx, true)
	if err != nil {
		return nil, err
	}
	for _, c := range clusters {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// 内部：模式分类
// ---------------------------------------------------------------------------

// classifyError 根据错误信息分类。
//
// Phase 1 规则分类：
//   - timeout
//   - tool_error
//   - llm_error
//   - json_parse
//   - rate_limit
//   - 其他
func classifyError(span *domain.Span) string {
	msg := span.ErrorMessage
	switch {
	case msg == "":
		return "unknown"
	case contains(msg, "timeout", "deadline"):
		return "timeout"
	case contains(msg, "tool", "function"):
		return "tool_error"
	case contains(msg, "rate_limit", "rate limit", "429"):
		return "rate_limit"
	case contains(msg, "json", "parse", "marshal"):
		return "json_parse"
	case span.Type == domain.SpanTypeLLM:
		return "llm_error"
	default:
		return "other_error"
	}
}

// contains 不区分大小写检查子串。
func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if indexOfIgnoreCase(s, sub) >= 0 {
			return true
		}
	}
	return false
}

func indexOfIgnoreCase(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	if len(sub) > len(s) {
		return -1
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			sc := s[i+j]
			tc := sub[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func generateDescription(name string, spans []*domain.Span) string {
	agents := make(map[string]bool)
	for _, s := range spans {
		agents[s.AgentName] = true
	}
	agentList := ""
	for a := range agents {
		if agentList != "" {
			agentList += ", "
		}
		agentList += a
	}
	return fmt.Sprintf("Pattern '%s' occurred in agents: %s", name, agentList)
}

func extractPattern(spans []*domain.Span) string {
	// 取前 3 条 error_message 作为样本
	samples := []string{}
	for i, s := range spans {
		if i >= 3 {
			break
		}
		if s.ErrorMessage != "" {
			samples = append(samples, s.ErrorMessage)
		}
	}
	b, _ := json.Marshal(samples)
	return string(b)
}

func extractTraceIDs(spans []*domain.Span, limit int) []string {
	ids := []string{}
	for i, s := range spans {
		if i >= limit {
			break
		}
		if s.TraceID != "" {
			ids = append(ids, s.TraceID)
		}
	}
	return ids
}

func generateSuggestion(name string) string {
	suggestions := map[string]string{
		"timeout":    "考虑增加超时时间或拆分子任务；或实现超时检测机制",
		"tool_error": "检查工具参数校验；考虑在 Prompt 中加入工具调用示例",
		"rate_limit": "实现指数退避重试；考虑降级到更便宜的模型",
		"json_parse": "使用 JSON Schema 校验；改用 OpenAI 的 response_format 参数",
		"llm_error":  "检查 LLM API 凭据和速率限制；增加重试机制",
		"other_error": "检查错误日志获取详细信息",
	}
	if s, ok := suggestions[name]; ok {
		return s
	}
	return "分析具体错误信息后改进"
}
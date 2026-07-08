// Package repository - ClickHouse Span 仓储实现。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/pkg/logger"
)

// ClickHouseSpanRepo 是 SpanRepository 的 ClickHouse 实现。
type ClickHouseSpanRepo struct {
	client *ClickHouseClient
	logger logger.Logger
}

// NewClickHouseSpanRepo 创建仓储实例。
func NewClickHouseSpanRepo(client *ClickHouseClient, log logger.Logger) *ClickHouseSpanRepo {
	return &ClickHouseSpanRepo{
		client: client,
		logger: log.WithFields(map[string]any{"component": "span_repo"}),
	}
}

// Client 返回底层 ClickHouse 客户端（用于服务层执行自定义 SQL）。
func (r *ClickHouseSpanRepo) Client() *ClickHouseClient {
	return r.client
}

// spanRow 是 ClickHouse 行结构（与表 schema 对应）。
type spanRow struct {
	Timestamp        time.Time `ch:"timestamp"`
	TraceID          string    `ch:"trace_id"`
	SpanID           string    `ch:"span_id"`
	ParentSpanID     string    `ch:"parent_span_id"`
	SessionID        string    `ch:"session_id"`
	UserID           string    `ch:"user_id"`
	AgentName        string    `ch:"agent_name"`
	ServiceName      string    `ch:"service_name"`
	Environment      string    `ch:"environment"`
	SpanType         string    `ch:"span_type"`
	SpanName         string    `ch:"span_name"`
	Status           string    `ch:"status"`
	Model            string    `ch:"model"`
	PromptTokens     uint32    `ch:"prompt_tokens"`
	CompletionTokens uint32    `ch:"completion_tokens"`
	TotalTokens      uint32    `ch:"total_tokens"`
	CostUSD          float64   `ch:"cost_usd"`
	FinishReason     string    `ch:"finish_reason"`
	ToolName         string    `ch:"tool_name"`
	ReasoningStep    uint16    `ch:"reasoning_step"`
	LatencyMs        uint32    `ch:"latency_ms"`
	InputPreview     string    `ch:"input_preview"`
	OutputPreview    string    `ch:"output_preview"`
	ErrorMessage     string    `ch:"error_message"`
	Attributes       string    `ch:"attributes"`
}

func (r spanRow) toDomain() *domain.Span {
	s := &domain.Span{
		ID:               r.SpanID,
		TraceID:          r.TraceID,
		ParentSpanID:     r.ParentSpanID,
		SessionID:        r.SessionID,
		UserID:           r.UserID,
		AgentName:        r.AgentName,
		ServiceName:      r.ServiceName,
		Environment:      r.Environment,
		Type:             domain.SpanType(r.SpanType),
		Name:             r.SpanName,
		Status:           domain.SpanStatus(r.Status),
		StartTime:        r.Timestamp,
		LatencyMs:        r.LatencyMs,
		Model:            r.Model,
		PromptTokens:     r.PromptTokens,
		CompletionTokens: r.CompletionTokens,
		TotalTokens:      r.TotalTokens,
		CostUSD:          r.CostUSD,
		FinishReason:     r.FinishReason,
		ToolName:         r.ToolName,
		ReasoningStep:    r.ReasoningStep,
		InputPreview:     r.InputPreview,
		OutputPreview:    r.OutputPreview,
		ErrorMessage:     r.ErrorMessage,
	}
	if !r.Timestamp.IsZero() && r.LatencyMs > 0 {
		s.EndTime = s.StartTime.Add(time.Duration(r.LatencyMs) * time.Millisecond)
	}
	if r.Attributes != "" {
		_ = json.Unmarshal([]byte(r.Attributes), &s.Attributes)
	}
	return s
}

// ---------------------------------------------------------------------------
// 写入
// ---------------------------------------------------------------------------

// Insert 插入单条 Span。
func (r *ClickHouseSpanRepo) Insert(ctx context.Context, span *domain.Span) error {
	return r.BatchInsert(ctx, []*domain.Span{span})
}

// BatchInsert 批量插入 Span。
//
// 使用 ClickHouse 原生批量协议（最高效）。
func (r *ClickHouseSpanRepo) BatchInsert(ctx context.Context, spans []*domain.Span) error {
	if len(spans) == 0 {
		return nil
	}

	const query = `INSERT INTO agent_spans (
		timestamp, trace_id, span_id, parent_span_id, session_id, user_id,
		agent_name, service_name, environment, span_type, span_name, status,
		model, prompt_tokens, completion_tokens, total_tokens, cost_usd, finish_reason,
		tool_name, reasoning_step, latency_ms,
		input_preview, output_preview, error_message, attributes
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	batch, err := r.client.Conn().PrepareBatch(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, span := range spans {
		attrs := "{}"
		if span.Attributes != nil {
			if b, err := json.Marshal(span.Attributes); err == nil {
				attrs = string(b)
			}
		}

		// 截断长字段（ClickHouse String 类型无长度限制，但过大影响性能）
		inputPreview := truncate(span.InputPreview, 4000)
		outputPreview := truncate(span.OutputPreview, 4000)
		errorMessage := truncate(span.ErrorMessage, 2000)

		env := span.Environment
		if env == "" {
			env = "production"
		}

		err := batch.Append(
			span.StartTime,
			span.TraceID,
			span.ID,
			span.ParentSpanID,
			span.SessionID,
			span.UserID,
			span.AgentName,
			span.ServiceName,
			env,
			string(span.Type),
			span.Name,
			string(span.Status),
			span.Model,
			span.PromptTokens,
			span.CompletionTokens,
			span.TotalTokens,
			span.CostUSD,
			span.FinishReason,
			span.ToolName,
			span.ReasoningStep,
			span.LatencyMs,
			inputPreview,
			outputPreview,
			errorMessage,
			attrs,
		)
		if err != nil {
			return fmt.Errorf("append span %s: %w", span.ID, err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	r.logger.Debugf("inserted %d spans", len(spans))
	return nil
}

// ---------------------------------------------------------------------------
// 查询
// ---------------------------------------------------------------------------

// GetByID 根据 Span ID 查询。
func (r *ClickHouseSpanRepo) GetByID(ctx context.Context, id string) (*domain.Span, error) {
	const query = `SELECT * FROM agent_spans WHERE span_id = ? LIMIT 1`

	var row spanRow
	err := r.client.QueryRow(ctx, &row, query, id)
	if err != nil {
		return nil, fmt.Errorf("query span: %w", err)
	}
	return row.toDomain(), nil
}

// GetByTraceID 查询 Trace 下所有 Span。
func (r *ClickHouseSpanRepo) GetByTraceID(ctx context.Context, traceID string) ([]*domain.Span, error) {
	const query = `SELECT * FROM agent_spans WHERE trace_id = ? ORDER BY timestamp ASC, span_id ASC`

	var spans []*domain.Span
	err := r.client.QueryRows(ctx, query, func(rows driver.Rows) error {
		var row spanRow
		if err := rows.ScanStruct(&row); err != nil {
			return err
		}
		spans = append(spans, row.toDomain())
		return nil
	}, traceID)

	if err != nil {
		return nil, fmt.Errorf("query trace: %w", err)
	}
	return spans, nil
}

// ListBySession 按会话查询。
func (r *ClickHouseSpanRepo) ListBySession(ctx context.Context, sessionID string, opts domain.ListOptions) ([]*domain.Span, error) {
	return r.list(ctx, &opts, "session_id = ?", sessionID)
}

// ListByUser 按用户查询。
func (r *ClickHouseSpanRepo) ListByUser(ctx context.Context, userID string, opts domain.ListOptions) ([]*domain.Span, error) {
	return r.list(ctx, &opts, "user_id = ?", userID)
}

// ListByAgent 按 Agent 查询。
func (r *ClickHouseSpanRepo) ListByAgent(ctx context.Context, agentName string, opts domain.ListOptions) ([]*domain.Span, error) {
	return r.list(ctx, &opts, "agent_name = ?", agentName)
}

// GetTraceTree 查询完整调用树。
func (r *ClickHouseSpanRepo) GetTraceTree(ctx context.Context, traceID string) (*domain.TraceTree, error) {
	spans, err := r.GetByTraceID(ctx, traceID)
	if err != nil {
		return nil, err
	}

	if len(spans) == 0 {
		return nil, nil
	}

	tree := &domain.TraceTree{
		TraceID:   traceID,
		SessionID: spans[0].SessionID,
		UserID:    spans[0].UserID,
		StartTime: spans[0].StartTime,
		EndTime:   spans[len(spans)-1].EndTime,
		AllSpans:  spans,
	}

	// 构建 Span Tree
	spanMap := make(map[string]*domain.Span, len(spans))
	for _, s := range spans {
		spanMap[s.ID] = s
	}

	for _, s := range spans {
		if s.ParentSpanID == "" {
			tree.Root = s
		} else if parent, ok := spanMap[s.ParentSpanID]; ok {
			// 父子关系通过 Attributes 字段传递（避免破坏领域模型）
			if parent.Attributes == nil {
				parent.Attributes = map[string]any{}
			}
			if _, exists := parent.Attributes["_children"]; !exists {
				parent.Attributes["_children"] = []*domain.Span{}
			}
			children := parent.Attributes["_children"].([]*domain.Span)
			children = append(children, s)
			parent.Attributes["_children"] = children
		}
		tree.Depth++
	}

	return tree, nil
}

// ---------------------------------------------------------------------------
// 内部
// ---------------------------------------------------------------------------

func (r *ClickHouseSpanRepo) list(ctx context.Context, opts *domain.ListOptions, whereClause string, whereArgs ...any) ([]*domain.Span, error) {
	var (
		conditions []string
		args       []any
	)

	conditions = append(conditions, whereClause)
	args = append(args, whereArgs...)

	if opts.From != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, *opts.From)
	}
	if opts.To != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, *opts.To)
	}
	if opts.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, string(opts.Status))
	}
	if opts.Type != "" {
		conditions = append(conditions, "span_type = ?")
		args = append(args, string(opts.Type))
	}

	orderBy := "timestamp"
	if opts.OrderBy != "" {
		orderBy = opts.OrderBy
	}
	order := "ASC"
	if opts.OrderDesc {
		order = "DESC"
	}

	limit := opts.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := fmt.Sprintf(
		"SELECT * FROM agent_spans WHERE %s ORDER BY %s %s LIMIT %d",
		strings.Join(conditions, " AND "),
		orderBy, order,
		limit,
	)

	var spans []*domain.Span
	err := r.client.QueryRows(ctx, query, func(rows driver.Rows) error {
		var row spanRow
		if err := rows.ScanStruct(&row); err != nil {
			return err
		}
		spans = append(spans, row.toDomain())
		return nil
	}, args...)

	if err != nil {
		return nil, fmt.Errorf("query list: %w", err)
	}
	return spans, nil
}

// truncate 安全截断字符串（按字符数，非字节数）。
func truncate(s string, maxChars int) string {
	if maxChars <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[:maxChars])
}
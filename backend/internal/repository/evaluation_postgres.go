// Package repository - PostgreSQL Evaluation 仓储实现。
package repository

import (
	"context"
	"fmt"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/jackc/pgx/v5"
)

// PostgresEvaluationRepo 是 EvaluationRepository 的 PostgreSQL 实现。
type PostgresEvaluationRepo struct {
	client *PostgresClient
	logger logger.Logger
}

// NewPostgresEvaluationRepo 创建仓储实例。
func NewPostgresEvaluationRepo(client *PostgresClient, log logger.Logger) *PostgresEvaluationRepo {
	return &PostgresEvaluationRepo{
		client: client,
		logger: log.WithFields(map[string]any{"component": "evaluation_repo"}),
	}
}

// evaluationRow 是数据库行结构。
type evaluationRow struct {
	ID             string  `db:"id"`
	SpanID         string  `db:"span_id"`
	TraceID        string  `db:"trace_id"`
	SessionID      string  `db:"session_id"`
	UserID         string  `db:"user_id"`
	AgentName      string  `db:"agent_name"`
	Accuracy       float32 `db:"accuracy"`
	Completeness   float32 `db:"completeness"`
	ToolSelection  float32 `db:"tool_selection"`
	ReasoningDepth float32 `db:"reasoning_depth"`
	Helpfulness    float32 `db:"helpfulness"`
	Overall        float32 `db:"overall"`
	Rationale      string  `db:"rationale"`
	JudgeModel     string  `db:"judge_model"`
	JudgePrompt    string  `db:"judge_prompt"`
	TriggerType    string  `db:"trigger_type"`
	SampleRate     float32 `db:"sample_rate"`
	CreatedAt      int64   `db:"created_at_unix"`
}

func (r evaluationRow) toDomain() *domain.Evaluation {
	return &domain.Evaluation{
		ID:             r.ID,
		SpanID:         r.SpanID,
		TraceID:        r.TraceID,
		SessionID:      r.SessionID,
		UserID:         r.UserID,
		AgentName:      r.AgentName,
		Accuracy:       r.Accuracy,
		Completeness:   r.Completeness,
		ToolSelection:  r.ToolSelection,
		ReasoningDepth: r.ReasoningDepth,
		Helpfulness:    r.Helpfulness,
		Overall:        r.Overall,
		Rationale:      r.Rationale,
		JudgeModel:     r.JudgeModel,
		JudgePrompt:    r.JudgePrompt,
		Trigger:        domain.EvaluationTrigger(r.TriggerType),
		SampleRate:     r.SampleRate,
		// CreatedAt: ...
	}
}

// ---------------------------------------------------------------------------
// 写入
// ---------------------------------------------------------------------------

// Insert 插入单条评估。
func (r *PostgresEvaluationRepo) Insert(ctx context.Context, eval *domain.Evaluation) error {
	return r.BatchInsert(ctx, []*domain.Evaluation{eval})
}

// BatchInsert 批量插入。
//
// 使用事务保证原子性。
func (r *PostgresEvaluationRepo) BatchInsert(ctx context.Context, evals []*domain.Evaluation) error {
	if len(evals) == 0 {
		return nil
	}

	tx, err := r.client.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const query = `INSERT INTO evaluations (
		span_id, trace_id, session_id, user_id, agent_name,
		accuracy, completeness, tool_selection, reasoning_depth, helpfulness, overall,
		rationale, judge_model, judge_prompt, trigger_type, sample_rate
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	for _, eval := range evals {
		_, err := tx.Exec(ctx, query,
			eval.SpanID,
			eval.TraceID,
			eval.SessionID,
			eval.UserID,
			eval.AgentName,
			eval.Accuracy,
			eval.Completeness,
			eval.ToolSelection,
			eval.ReasoningDepth,
			eval.Helpfulness,
			eval.Overall,
			eval.Rationale,
			eval.JudgeModel,
			eval.JudgePrompt,
			string(eval.Trigger),
			eval.SampleRate,
		)
		if err != nil {
			return fmt.Errorf("insert eval %s: %w", eval.SpanID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	r.logger.Debugf("inserted %d evaluations", len(evals))
	return nil
}

// ---------------------------------------------------------------------------
// 查询
// ---------------------------------------------------------------------------

// GetByID 根据 ID 查询。
func (r *PostgresEvaluationRepo) GetByID(ctx context.Context, id string) (*domain.Evaluation, error) {
	const query = `SELECT
		id::text, span_id, trace_id, session_id::text, user_id, agent_name,
		accuracy, completeness, tool_selection, reasoning_depth, helpfulness, overall,
		rationale, judge_model, judge_prompt, trigger_type, sample_rate,
		EXTRACT(EPOCH FROM created_at)::bigint AS created_at_unix
	FROM evaluations WHERE id = $1 LIMIT 1`

	row := evaluationRow{}
	err := r.queryRow(ctx, query, &row, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return row.toDomain(), nil
}

// GetBySpanID 根据 Span ID 查询。
func (r *PostgresEvaluationRepo) GetBySpanID(ctx context.Context, spanID string) (*domain.Evaluation, error) {
	const query = `SELECT
		id::text, span_id, trace_id, session_id::text, user_id, agent_name,
		accuracy, completeness, tool_selection, reasoning_depth, helpfulness, overall,
		rationale, judge_model, judge_prompt, trigger_type, sample_rate,
		EXTRACT(EPOCH FROM created_at)::bigint AS created_at_unix
	FROM evaluations WHERE span_id = $1 LIMIT 1`

	row := evaluationRow{}
	err := r.queryRow(ctx, query, &row, spanID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return row.toDomain(), nil
}

// ListBySession 查询会话下所有评估。
func (r *PostgresEvaluationRepo) ListBySession(ctx context.Context, sessionID string) ([]*domain.Evaluation, error) {
	const query = `SELECT
		id::text, span_id, trace_id, session_id::text, user_id, agent_name,
		accuracy, completeness, tool_selection, reasoning_depth, helpfulness, overall,
		rationale, judge_model, judge_prompt, trigger_type, sample_rate,
		EXTRACT(EPOCH FROM created_at)::bigint AS created_at_unix
	FROM evaluations WHERE session_id = $1 ORDER BY created_at DESC`

	return r.queryList(ctx, query, sessionID)
}

// ListByAgent 查询 Agent 评估历史。
func (r *PostgresEvaluationRepo) ListByAgent(ctx context.Context, agentName string, opts domain.ListOptions) ([]*domain.Evaluation, error) {
	const query = `SELECT
		id::text, span_id, trace_id, session_id::text, user_id, agent_name,
		accuracy, completeness, tool_selection, reasoning_depth, helpfulness, overall,
		rationale, judge_model, judge_prompt, trigger_type, sample_rate,
		EXTRACT(EPOCH FROM created_at)::bigint AS created_at_unix
	FROM evaluations WHERE agent_name = $1 ORDER BY created_at DESC LIMIT 100`

	return r.queryList(ctx, query, agentName)
}

// AverageScores 聚合查询各维度平均分。
func (r *PostgresEvaluationRepo) AverageScores(
	ctx context.Context,
	agentName string,
	window domain.TimeWindow,
) (map[domain.EvaluationDimension]float32, error) {
	const query = `SELECT
		AVG(accuracy)::real       AS accuracy,
		AVG(completeness)::real   AS completeness,
		AVG(tool_selection)::real AS tool_selection,
		AVG(reasoning_depth)::real AS reasoning_depth,
		AVG(helpfulness)::real    AS helpfulness
	FROM evaluations
	WHERE agent_name = $1
	  AND created_at >= $2
	  AND created_at <= $3`

	var result struct {
		Accuracy       float32 `db:"accuracy"`
		Completeness   float32 `db:"completeness"`
		ToolSelection  float32 `db:"tool_selection"`
		ReasoningDepth float32 `db:"reasoning_depth"`
		Helpfulness    float32 `db:"helpfulness"`
	}

	err := r.queryRow(ctx, query, &result, agentName, window.From, window.To)
	if err != nil {
		return nil, err
	}

	return map[domain.EvaluationDimension]float32{
		domain.DimensionAccuracy:       result.Accuracy,
		domain.DimensionCompleteness:   result.Completeness,
		domain.DimensionToolSelection:  result.ToolSelection,
		domain.DimensionReasoningDepth: result.ReasoningDepth,
		domain.DimensionHelpfulness:    result.Helpfulness,
	}, nil
}

// ---------------------------------------------------------------------------
// 内部
// ---------------------------------------------------------------------------

func (r *PostgresEvaluationRepo) queryRow(ctx context.Context, query string, dest any, args ...any) error {
	rows, err := r.client.Pool().Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return pgx.ErrNoRows
	}

	return rows.Scan(dest)
}

func (r *PostgresEvaluationRepo) queryList(ctx context.Context, query string, args ...any) ([]*domain.Evaluation, error) {
	rows, err := r.client.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query list: %w", err)
	}
	defer rows.Close()

	var evals []*domain.Evaluation
	for rows.Next() {
		var row evaluationRow
		if err := rows.ScanStruct(&row); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		evals = append(evals, row.toDomain())
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return evals, nil
}
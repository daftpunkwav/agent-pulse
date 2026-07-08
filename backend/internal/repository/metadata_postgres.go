// Package repository - PostgreSQL Metadata 仓储实现（Harness/AB Test/Failure Cluster）。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/jackc/pgx/v5"
)

// PostgresMetadataRepo MetadataRepository 的 PostgreSQL 实现。
type PostgresMetadataRepo struct {
	client *PostgresClient
	logger logger.Logger
}

// NewPostgresMetadataRepo 创建仓储实例。
func NewPostgresMetadataRepo(client *PostgresClient, log logger.Logger) *PostgresMetadataRepo {
	return &PostgresMetadataRepo{
		client: client,
		logger: log.WithFields(map[string]any{"component": "metadata_repo"}),
	}
}

// ===========================================================================
// Harness 配置
// ===========================================================================

// CreateHarnessVersion 创建新版本。
//
// 自动计算下一个版本号（agent_name 下递增）。
func (r *PostgresMetadataRepo) CreateHarnessVersion(ctx context.Context, hc *domain.HarnessConfig) error {
	tx, err := r.client.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 查询下一个版本号
	var maxVersion int
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM harness_configs WHERE agent_name = $1`,
		hc.AgentName,
	).Scan(&maxVersion)
	if err != nil {
		return fmt.Errorf("query max version: %w", err)
	}

	hc.Version = maxVersion + 1
	if hc.Status == "" {
		hc.Status = domain.HarnessArchived
	}

	const query = `INSERT INTO harness_configs (
		agent_name, version, config_yaml, config_hash,
		status, traffic_percent, notes, created_by, created_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	RETURNING id, created_at`

	now := time.Now()
	err = tx.QueryRow(ctx, query,
		hc.AgentName,
		hc.Version,
		hc.ConfigYAML,
		hc.ConfigHash,
		string(hc.Status),
		hc.TrafficPercent,
		hc.Notes,
		hc.CreatedBy,
		now,
	).Scan(&hc.ID, &hc.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert harness: %w", err)
	}

	return tx.Commit(ctx)
}

// GetHarnessVersion 查询指定版本。
func (r *PostgresMetadataRepo) GetHarnessVersion(ctx context.Context, agentName string, version int) (*domain.HarnessConfig, error) {
	const query = `SELECT
		id::text, agent_name, version, config_yaml, config_hash,
		status, traffic_percent, COALESCE(notes, ''), COALESCE(created_by, ''),
		created_at, promoted_at
	FROM harness_configs WHERE agent_name = $1 AND version = $2 LIMIT 1`

	var row struct {
		ID             string         `db:"id"`
		AgentName      string         `db:"agent_name"`
		Version        int            `db:"version"`
		ConfigYAML     string         `db:"config_yaml"`
		ConfigHash     string         `db:"config_hash"`
		Status         string         `db:"status"`
		TrafficPercent int            `db:"traffic_percent"`
		Notes          string         `db:"notes"`
		CreatedBy      string         `db:"created_by"`
		CreatedAt      time.Time      `db:"created_at"`
		PromotedAt     *time.Time     `db:"promoted_at"`
	}

	err := r.scanOne(ctx, query, &row, agentName, version)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &domain.HarnessConfig{
		ID:             row.ID,
		AgentName:      row.AgentName,
		Version:        row.Version,
		ConfigYAML:     row.ConfigYAML,
		ConfigHash:     row.ConfigHash,
		Status:         domain.HarnessStatus(row.Status),
		TrafficPercent: row.TrafficPercent,
		Notes:          row.Notes,
		CreatedBy:      row.CreatedBy,
		CreatedAt:      row.CreatedAt,
		PromotedAt:     row.PromotedAt,
	}, nil
}

// ListHarnessVersions 列出 Agent 所有版本。
func (r *PostgresMetadataRepo) ListHarnessVersions(ctx context.Context, agentName string) ([]*domain.HarnessConfig, error) {
	const query = `SELECT
		id::text, agent_name, version, config_yaml, config_hash,
		status, traffic_percent, COALESCE(notes, ''), COALESCE(created_by, ''),
		created_at, promoted_at
	FROM harness_configs WHERE agent_name = $1 ORDER BY version DESC`

	return r.scanHarnessList(ctx, query, agentName)
}

// UpdateHarnessStatus 更新版本状态。
//
// 事务保证：将原 production 版本降级为 archived，将新版本提升为 production。
func (r *PostgresMetadataRepo) UpdateHarnessStatus(ctx context.Context, agentName string, version int, status domain.HarnessStatus) error {
	tx, err := r.client.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if status == domain.HarnessProduction {
		// 将该 agent 现有 production 版本降级
		_, err = tx.Exec(ctx,
			`UPDATE harness_configs SET status = 'archived' WHERE agent_name = $1 AND status = 'production'`,
			agentName,
		)
		if err != nil {
			return fmt.Errorf("demote old production: %w", err)
		}
	}

	// 更新目标版本
	_, err = tx.Exec(ctx,
		`UPDATE harness_configs SET status = $1, promoted_at = $2 WHERE agent_name = $3 AND version = $4`,
		string(status), time.Now(), agentName, version,
	)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return tx.Commit(ctx)
}

// ===========================================================================
// A/B Test
// ===========================================================================

// CreateABTest 创建 A/B 测试。
func (r *PostgresMetadataRepo) CreateABTest(ctx context.Context, ab *domain.ABTest) error {
	const query = `INSERT INTO ab_tests (
		name, agent_name, control_version, treatment_version,
		traffic_percent, status, started_at, metadata
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id, created_at`

	metadataJSON, _ := json.Marshal(ab.Metadata)
	if metadataJSON == nil {
		metadataJSON = []byte("{}")
	}

	err := r.client.Pool().QueryRow(ctx, query,
		ab.Name,
		ab.AgentName,
		ab.ControlVersion,
		ab.TreatmentVersion,
		ab.TrafficPercent,
		string(ab.Status),
		ab.StartedAt,
		string(metadataJSON),
	).Scan(&ab.ID, &ab.CreatedAt)

	if err != nil {
		return fmt.Errorf("insert ab test: %w", err)
	}

	return nil
}

// GetABTest 查询 A/B 测试。
func (r *PostgresMetadataRepo) GetABTest(ctx context.Context, id string) (*domain.ABTest, error) {
	const query = `SELECT
		id::text, name, agent_name, control_version, treatment_version,
		traffic_percent, status, started_at, ended_at, result, metadata, created_at
	FROM ab_tests WHERE id = $1 LIMIT 1`

	var row struct {
		ID               string          `db:"id"`
		Name             string          `db:"name"`
		AgentName        string          `db:"agent_name"`
		ControlVersion   int             `db:"control_version"`
		TreatmentVersion int             `db:"treatment_version"`
		TrafficPercent   int             `db:"traffic_percent"`
		Status           string          `db:"status"`
		StartedAt        time.Time       `db:"started_at"`
		EndedAt          *time.Time      `db:"ended_at"`
		Result           []byte          `db:"result"`
		Metadata         []byte          `db:"metadata"`
		CreatedAt        time.Time       `db:"created_at"`
	}

	err := r.scanOne(ctx, query, &row, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	ab := &domain.ABTest{
		ID:               row.ID,
		Name:             row.Name,
		AgentName:        row.AgentName,
		ControlVersion:   row.ControlVersion,
		TreatmentVersion: row.TreatmentVersion,
		TrafficPercent:   row.TrafficPercent,
		Status:           domain.ABTestStatus(row.Status),
		StartedAt:        row.StartedAt,
		EndedAt:          row.EndedAt,
		CreatedAt:        row.CreatedAt,
	}

	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &ab.Metadata)
	}

	if len(row.Result) > 0 {
		var result domain.ABTestResult
		if err := json.Unmarshal(row.Result, &result); err == nil {
			ab.Result = &result
		}
	}

	return ab, nil
}

// ListABTests 列出 A/B 测试。
func (r *PostgresMetadataRepo) ListABTests(ctx context.Context, opts domain.ListOptions) ([]*domain.ABTest, error) {
	query := `SELECT
		id::text, name, agent_name, control_version, treatment_version,
		traffic_percent, status, started_at, ended_at, result, metadata, created_at
	FROM ab_tests ORDER BY created_at DESC LIMIT 100`

	rows, err := r.client.Pool().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query ab tests: %w", err)
	}
	defer rows.Close()

	var abs []*domain.ABTest
	for rows.Next() {
		var row struct {
			ID               string          `db:"id"`
			Name             string          `db:"name"`
			AgentName        string          `db:"agent_name"`
			ControlVersion   int             `db:"control_version"`
			TreatmentVersion int             `db:"treatment_version"`
			TrafficPercent   int             `db:"traffic_percent"`
			Status           string          `db:"status"`
			StartedAt        time.Time       `db:"started_at"`
			EndedAt          *time.Time      `db:"ended_at"`
			Result           []byte          `db:"result"`
			Metadata         []byte          `db:"metadata"`
			CreatedAt        time.Time       `db:"created_at"`
		}
		if err := rows.ScanStruct(&row); err != nil {
			return nil, err
		}

		ab := &domain.ABTest{
			ID:               row.ID,
			Name:             row.Name,
			AgentName:        row.AgentName,
			ControlVersion:   row.ControlVersion,
			TreatmentVersion: row.TreatmentVersion,
			TrafficPercent:   row.TrafficPercent,
			Status:           domain.ABTestStatus(row.Status),
			StartedAt:        row.StartedAt,
			EndedAt:          row.EndedAt,
			CreatedAt:        row.CreatedAt,
		}
		if len(row.Metadata) > 0 {
			_ = json.Unmarshal(row.Metadata, &ab.Metadata)
		}
		if len(row.Result) > 0 {
			var result domain.ABTestResult
			if err := json.Unmarshal(row.Result, &result); err == nil {
				ab.Result = &result
			}
		}
		abs = append(abs, ab)
	}

	return abs, rows.Err()
}

// UpdateABTestResult 更新 A/B 测试结果。
func (r *PostgresMetadataRepo) UpdateABTestResult(ctx context.Context, id string, result *domain.ABTestResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	_, err = r.client.Pool().Exec(ctx,
		`UPDATE ab_tests SET status = $1, ended_at = $2, result = $3 WHERE id = $4`,
		string(domain.ABTestCompleted), result.ComputedAt, string(resultJSON), id,
	)
	return err
}

// ===========================================================================
// Failure Cluster
// ===========================================================================

// InsertFailureCluster 插入聚类结果。
func (r *PostgresMetadataRepo) InsertFailureCluster(ctx context.Context, cluster *domain.FailureCluster) error {
	tracesJSON, _ := json.Marshal(cluster.ExampleTraces)
	if tracesJSON == nil {
		tracesJSON = []byte("[]")
	}
	metadataJSON, _ := json.Marshal(cluster.Metadata)
	if metadataJSON == nil {
		metadataJSON = []byte("{}")
	}

	const query = `INSERT INTO failure_clusters (
		cluster_name, description, trace_count, percentage,
		common_pattern, suggestion, example_traces, metadata, is_active
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	RETURNING id, created_at, updated_at`

	err := r.client.Pool().QueryRow(ctx, query,
		cluster.Name,
		cluster.Description,
		cluster.TraceCount,
		cluster.Percentage,
		cluster.CommonPattern,
		cluster.Suggestion,
		string(tracesJSON),
		string(metadataJSON),
		cluster.IsActive,
	).Scan(&cluster.ID, &cluster.CreatedAt, &cluster.UpdatedAt)

	return err
}

// ListFailureClusters 列出聚类结果。
func (r *PostgresMetadataRepo) ListFailureClusters(ctx context.Context, activeOnly bool) ([]*domain.FailureCluster, error) {
	query := `SELECT
		id::text, cluster_name, description, trace_count, percentage,
		common_pattern, suggestion, example_traces, metadata,
		is_active, created_at, updated_at
	FROM failure_clusters`

	if activeOnly {
		query += ` WHERE is_active = true`
	}
	query += ` ORDER BY trace_count DESC LIMIT 100`

	rows, err := r.client.Pool().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query clusters: %w", err)
	}
	defer rows.Close()

	var clusters []*domain.FailureCluster
	for rows.Next() {
		var row struct {
			ID            string    `db:"id"`
			Name          string    `db:"cluster_name"`
			Description   string    `db:"description"`
			TraceCount    int       `db:"trace_count"`
			Percentage    float32   `db:"percentage"`
			CommonPattern string    `db:"common_pattern"`
			Suggestion    string    `db:"suggestion"`
			ExampleTraces []byte    `db:"example_traces"`
			Metadata      []byte    `db:"metadata"`
			IsActive      bool      `db:"is_active"`
			CreatedAt     time.Time `db:"created_at"`
			UpdatedAt     time.Time `db:"updated_at"`
		}
		if err := rows.ScanStruct(&row); err != nil {
			return nil, err
		}

		cluster := &domain.FailureCluster{
			ID:            row.ID,
			Name:          row.Name,
			Description:   row.Description,
			TraceCount:    row.TraceCount,
			Percentage:    row.Percentage,
			CommonPattern: row.CommonPattern,
			Suggestion:    row.Suggestion,
			IsActive:      row.IsActive,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
		}
		if len(row.ExampleTraces) > 0 {
			_ = json.Unmarshal(row.ExampleTraces, &cluster.ExampleTraces)
		}
		if len(row.Metadata) > 0 {
			_ = json.Unmarshal(row.Metadata, &cluster.Metadata)
		}
		clusters = append(clusters, cluster)
	}

	return clusters, rows.Err()
}

// ---------------------------------------------------------------------------
// 内部辅助
// ---------------------------------------------------------------------------

func (r *PostgresMetadataRepo) scanOne(ctx context.Context, query string, dest any, args ...any) error {
	rows, err := r.client.Pool().Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return pgx.ErrNoRows
	}

	return rows.ScanStruct(dest)
}

func (r *PostgresMetadataRepo) scanHarnessList(ctx context.Context, query string, args ...any) ([]*domain.HarnessConfig, error) {
	rows, err := r.client.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query harness list: %w", err)
	}
	defer rows.Close()

	var configs []*domain.HarnessConfig
	for rows.Next() {
		var row struct {
			ID             string     `db:"id"`
			AgentName      string     `db:"agent_name"`
			Version        int        `db:"version"`
			ConfigYAML     string     `db:"config_yaml"`
			ConfigHash     string     `db:"config_hash"`
			Status         string     `db:"status"`
			TrafficPercent int        `db:"traffic_percent"`
			Notes          string     `db:"notes"`
			CreatedBy      string     `db:"created_by"`
			CreatedAt      time.Time  `db:"created_at"`
			PromotedAt     *time.Time `db:"promoted_at"`
		}
		if err := rows.ScanStruct(&row); err != nil {
			return nil, err
		}
		configs = append(configs, &domain.HarnessConfig{
			ID:             row.ID,
			AgentName:      row.AgentName,
			Version:        row.Version,
			ConfigYAML:     row.ConfigYAML,
			ConfigHash:     row.ConfigHash,
			Status:         domain.HarnessStatus(row.Status),
			TrafficPercent: row.TrafficPercent,
			Notes:          row.Notes,
			CreatedBy:      row.CreatedBy,
			CreatedAt:      row.CreatedAt,
			PromotedAt:     row.PromotedAt,
		})
	}

	return configs, rows.Err()
}
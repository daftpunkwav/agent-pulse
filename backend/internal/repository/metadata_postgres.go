// Package repository - PostgreSQL Metadata 仓储实现（Harness/AB Test/Failure Cluster）。
package repository

import (
	"context"
	"encoding/json"
	"errors"
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
func (r *PostgresMetadataRepo) CreateHarnessVersion(ctx context.Context, hc *domain.HarnessConfig) error {
	tx, err := r.client.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

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

	var (
		id             string
		agName         string
		v              int
		yaml           string
		hash           string
		status         string
		trafficPercent int
		notes          string
		createdBy      string
		createdAt      time.Time
		promotedAt     *time.Time
	)

	err := r.client.Pool().QueryRow(ctx, query, agentName, version).Scan(
		&id, &agName, &v, &yaml, &hash,
		&status, &trafficPercent, &notes, &createdBy,
		&createdAt, &promotedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &domain.HarnessConfig{
		ID:             id,
		AgentName:      agName,
		Version:        v,
		ConfigYAML:     yaml,
		ConfigHash:     hash,
		Status:         domain.HarnessStatus(status),
		TrafficPercent: trafficPercent,
		Notes:          notes,
		CreatedBy:      createdBy,
		CreatedAt:      createdAt,
		PromotedAt:     promotedAt,
	}, nil
}

// ListHarnessVersions 列出 Agent 所有版本。
func (r *PostgresMetadataRepo) ListHarnessVersions(ctx context.Context, agentName string) ([]*domain.HarnessConfig, error) {
	const query = `SELECT
		id::text, agent_name, version, config_yaml, config_hash,
		status, traffic_percent, COALESCE(notes, ''), COALESCE(created_by, ''),
		created_at, promoted_at
	FROM harness_configs WHERE agent_name = $1 ORDER BY version DESC`

	rows, err := r.client.Pool().Query(ctx, query, agentName)
	if err != nil {
		return nil, fmt.Errorf("query harness list: %w", err)
	}
	defer rows.Close()

	var configs []*domain.HarnessConfig
	for rows.Next() {
		var (
			id, agName, yaml, hash, status, notes, createdBy string
			v, trafficPercent                                int
			createdAt                                        time.Time
			promotedAt                                       *time.Time
		)
		if err := rows.Scan(
			&id, &agName, &v, &yaml, &hash,
			&status, &trafficPercent, &notes, &createdBy,
			&createdAt, &promotedAt,
		); err != nil {
			return nil, err
		}
		configs = append(configs, &domain.HarnessConfig{
			ID:             id,
			AgentName:      agName,
			Version:        v,
			ConfigYAML:     yaml,
			ConfigHash:     hash,
			Status:         domain.HarnessStatus(status),
			TrafficPercent: trafficPercent,
			Notes:          notes,
			CreatedBy:      createdBy,
			CreatedAt:      createdAt,
			PromotedAt:     promotedAt,
		})
	}

	if configs == nil {
		configs = []*domain.HarnessConfig{}
	}
	return configs, rows.Err()
}

// UpdateHarnessStatus 更新版本状态。
func (r *PostgresMetadataRepo) UpdateHarnessStatus(ctx context.Context, agentName string, version int, status domain.HarnessStatus) error {
	tx, err := r.client.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if status == domain.HarnessProduction {
		_, err = tx.Exec(ctx,
			`UPDATE harness_configs SET status = 'archived' WHERE agent_name = $1 AND status = 'production'`,
			agentName,
		)
		if err != nil {
			return fmt.Errorf("demote old production: %w", err)
		}
	}

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

	var (
		abID, name, agentName, status string
		controlV, treatmentV, traffic int
		startedAt                    time.Time
		endedAt                      *time.Time
		resultJSON                   []byte
		metadataJSON                 []byte
		createdAt                    time.Time
	)

	err := r.client.Pool().QueryRow(ctx, query, id).Scan(
		&abID, &name, &agentName, &controlV, &treatmentV,
		&traffic, &status, &startedAt, &endedAt, &resultJSON, &metadataJSON, &createdAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	ab := &domain.ABTest{
		ID:               abID,
		Name:             name,
		AgentName:        agentName,
		ControlVersion:   controlV,
		TreatmentVersion: treatmentV,
		TrafficPercent:   traffic,
		Status:           domain.ABTestStatus(status),
		StartedAt:        startedAt,
		EndedAt:          endedAt,
		CreatedAt:        createdAt,
	}

	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &ab.Metadata)
	}
	if len(resultJSON) > 0 {
		var result domain.ABTestResult
		if err := json.Unmarshal(resultJSON, &result); err == nil {
			ab.Result = &result
		}
	}

	return ab, nil
}

// ListABTests 列出 A/B 测试。
func (r *PostgresMetadataRepo) ListABTests(ctx context.Context, opts domain.ListOptions) ([]*domain.ABTest, error) {
	const query = `SELECT
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
		var (
			abID, name, agentName, status string
			controlV, treatmentV, traffic int
			startedAt                    time.Time
			endedAt                      *time.Time
			resultJSON                   []byte
			metadataJSON                 []byte
			createdAt                    time.Time
		)
		if err := rows.Scan(
			&abID, &name, &agentName, &controlV, &treatmentV,
			&traffic, &status, &startedAt, &endedAt, &resultJSON, &metadataJSON, &createdAt,
		); err != nil {
			return nil, err
		}

		ab := &domain.ABTest{
			ID:               abID,
			Name:             name,
			AgentName:        agentName,
			ControlVersion:   controlV,
			TreatmentVersion: treatmentV,
			TrafficPercent:   traffic,
			Status:           domain.ABTestStatus(status),
			StartedAt:        startedAt,
			EndedAt:          endedAt,
			CreatedAt:        createdAt,
		}
		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &ab.Metadata)
		}
		if len(resultJSON) > 0 {
			var result domain.ABTestResult
			if err := json.Unmarshal(resultJSON, &result); err == nil {
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
		var (
			id, name, description, commonPattern, suggestion string
			traceCount                                      int
			percentage                                      float32
			exampleTracesJSON                               []byte
			metadataJSON                                    []byte
			isActive                                        bool
			createdAt, updatedAt                            time.Time
		)
		if err := rows.Scan(
			&id, &name, &description, &traceCount, &percentage,
			&commonPattern, &suggestion, &exampleTracesJSON, &metadataJSON,
			&isActive, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}

		cluster := &domain.FailureCluster{
			ID:            id,
			Name:          name,
			Description:   description,
			TraceCount:    traceCount,
			Percentage:    percentage,
			CommonPattern: commonPattern,
			Suggestion:    suggestion,
			IsActive:      isActive,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		}
		if len(exampleTracesJSON) > 0 {
			_ = json.Unmarshal(exampleTracesJSON, &cluster.ExampleTraces)
		}
		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &cluster.Metadata)
		}
		clusters = append(clusters, cluster)
	}

	return clusters, rows.Err()
}

// GetFailureClusterByID 按 ID 查询单个聚类。
func (r *PostgresMetadataRepo) GetFailureClusterByID(ctx context.Context, id string) (*domain.FailureCluster, error) {
	query := `SELECT
		id::text, cluster_name, description, trace_count, percentage,
		common_pattern, suggestion, example_traces, metadata,
		is_active, created_at, updated_at
	FROM failure_clusters
	WHERE id = $1`

	var (
		clusterID, name, description, commonPattern, suggestion string
		traceCount                                              int
		percentage                                              float32
		exampleTracesJSON                                       []byte
		metadataJSON                                            []byte
		isActive                                                bool
		createdAt, updatedAt                                    time.Time
	)

	err := r.client.Pool().QueryRow(ctx, query, id).Scan(
		&clusterID, &name, &description, &traceCount, &percentage,
		&commonPattern, &suggestion, &exampleTracesJSON, &metadataJSON,
		&isActive, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get cluster by id: %w", err)
	}

	cluster := &domain.FailureCluster{
		ID:            clusterID,
		Name:          name,
		Description:   description,
		TraceCount:    traceCount,
		Percentage:    percentage,
		CommonPattern: commonPattern,
		Suggestion:    suggestion,
		IsActive:      isActive,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
	if len(exampleTracesJSON) > 0 {
		_ = json.Unmarshal(exampleTracesJSON, &cluster.ExampleTraces)
	}
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &cluster.Metadata)
	}
	return cluster, nil
}
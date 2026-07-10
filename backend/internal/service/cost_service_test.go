// Package service - CostService 集成测试。
//
// 验证 API（Breakdown/TotalCost/Timeline）→ Service（CostService）→
// Repository（domain.ClickHouseQueryExecutor）跨层调用正确性。
//
// 测试策略：使用 domain.ClickHouseQueryExecutor mock 验证业务逻辑，
// 不依赖真实 ClickHouse。
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/service"
)

// ===== Mock domain.ClickHouseQueryExecutor =====

// mockRows 实现 domain.Rows。
type mockRows struct {
	rows   [][]any
	cursor int
	err    error
}

func newMockRows(rows [][]any) *mockRows { return &mockRows{rows: rows} }
func (r *mockRows) Next() bool            { r.cursor++; return r.cursor <= len(r.rows) }
func (r *mockRows) Scan(dest ...any) error {
	// After Next() returns false, Scan() may still be called by some implementations.
	// Gracefully skip when cursor is out of range.
	if r.cursor < 1 || r.cursor > len(r.rows) {
		return nil
	}
	row := r.rows[r.cursor-1]
	for i, d := range dest {
		if i < len(row) {
			switch ptr := d.(type) {
			case *string:
				if s, ok := row[i].(string); ok { *ptr = s }
			case *float64:
				if f, ok := row[i].(float64); ok { *ptr = f }
			case *uint64:
				if u, ok := row[i].(uint64); ok { *ptr = u }
			case *time.Time:
				if t, ok := row[i].(time.Time); ok { *ptr = t }
			}
		}
	}
	return r.err
}
func (r *mockRows) Err() error { return r.err }
func (r *mockRows) Close() error { return nil }

// mockRow 实现 domain.Row。
type mockRow struct {
	values []any
	err    error
}

func newMockRow(values ...any) *mockRow { return &mockRow{values: values} }
func (r *mockRow) Scan(dest ...any) error {
	for i, v := range r.values {
		if i < len(dest) {
			switch ptr := dest[i].(type) {
			case *string:
				if s, ok := v.(string); ok { *ptr = s }
			case *float64:
				if f, ok := v.(float64); ok { *ptr = f }
			case *uint64:
				if u, ok := v.(uint64); ok { *ptr = u }
			}
		}
	}
	return r.err
}

// mockConn 实现 domain.Conn。
type mockConn struct{ execFn func(ctx context.Context, query string, args ...any) error }

func (c *mockConn) Exec(ctx context.Context, query string, args ...any) error {
	if c.execFn != nil {
		return c.execFn(ctx, query, args...)
	}
	return nil
}

// mockQueryExecutor 实现 domain.ClickHouseQueryExecutor。
type mockQueryExecutor struct {
	rowsData  [][]any
	rowData   []any
	rowsErr   error
	rowErr    error
	execFn    func(ctx context.Context, query string, args ...any) error
}

func newMockQueryExecutor(rowsData [][]any) *mockQueryExecutor {
	return &mockQueryExecutor{rowsData: rowsData}
}

func (m *mockQueryExecutor) QueryRows(ctx context.Context, query string, scanFn func(domain.Rows) error, args ...any) error {
	if m.rowsErr != nil {
		return m.rowsErr
	}
	rows := newMockRows(m.rowsData)
	for rows.Next() {
		if err := scanFn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (m *mockQueryExecutor) QueryRow(ctx context.Context, query string, scanFn func(domain.Row) error, args ...any) error {
	if m.rowErr != nil {
		return m.rowErr
	}
	row := newMockRow(m.rowData...)
	return scanFn(row)
}

func (m *mockQueryExecutor) Conn() domain.Conn {
	return &mockConn{execFn: m.execFn}
}

// ===== Integration Tests =====

func TestNewCostService(t *testing.T) {
	executor := newMockQueryExecutor(nil)
	log := &testLogger{t: t}

	svc := service.NewCostService(executor, nil, nil, log)
	if svc == nil {
		t.Fatal("NewCostService returned nil")
	}
}

func TestCostService_Breakdown_NilExecutor_ReturnsError(t *testing.T) {
	log := &testLogger{t: t}
	svc := service.NewCostService(nil, nil, nil, log)

	_, err := svc.Breakdown(context.Background(), domain.TimeWindow{}, nil, 10)
	if err == nil {
		t.Error("Breakdown with nil executor should return error")
	}
}

func TestCostService_TotalCost_NilExecutor_ReturnsError(t *testing.T) {
	log := &testLogger{t: t}
	svc := service.NewCostService(nil, nil, nil, log)

	_, _, err := svc.TotalCost(context.Background(), domain.TimeWindow{})
	if err == nil {
		t.Error("TotalCost with nil executor should return error")
	}
}

func TestCostService_Timeline_NilExecutor_ReturnsError(t *testing.T) {
	log := &testLogger{t: t}
	svc := service.NewCostService(nil, nil, nil, log)

	_, err := svc.Timeline(context.Background(), domain.TimeWindow{}, "hour")
	if err == nil {
		t.Error("Timeline with nil executor should return error")
	}
}

func TestCostService_Breakdown_QueryRowsError_ReturnsError(t *testing.T) {
	executor := &mockQueryExecutor{rowsErr: errors.New("clickhouse connection lost")}
	log := &testLogger{t: t}
	svc := service.NewCostService(executor, nil, nil, log)

	_, err := svc.Breakdown(context.Background(), domain.TimeWindow{}, []domain.CostDimension{domain.DimensionUser}, 10)
	if err == nil {
		t.Error("Breakdown should return error when QueryRows fails")
	}
}

func TestCostService_Breakdown_EmptyWindow_ReturnsResults(t *testing.T) {
	executor := newMockQueryExecutor(nil) // no rows = empty result
	log := &testLogger{t: t}
	svc := service.NewCostService(executor, nil, nil, log)

	results, err := svc.Breakdown(context.Background(), domain.TimeWindow{}, nil, 10)
	if err != nil {
		t.Fatalf("Breakdown with no rows should not error: %v", err)
	}
	if len(results) != len(domain.AllCostDimensions()) {
		t.Errorf("expected %d dimension results, got %d", len(domain.AllCostDimensions()), len(results))
	}

	// Each dimension result should have empty items
	for _, r := range results {
		if len(r.Items) != 0 {
			t.Errorf("dimension %s: expected 0 items, got %d", r.Dimension, len(r.Items))
		}
		if r.TotalUSD != 0 || r.TotalTokens != 0 {
			t.Errorf("dimension %s: expected zero totals", r.Dimension)
		}
	}
}

func TestCostService_TotalCost_QueryRowError_ReturnsError(t *testing.T) {
	executor := &mockQueryExecutor{rowErr: errors.New("query timeout")}
	log := &testLogger{t: t}
	svc := service.NewCostService(executor, nil, nil, log)

	_, _, err := svc.TotalCost(context.Background(), domain.TimeWindow{})
	if err == nil {
		t.Error("TotalCost should return error when QueryRow fails")
	}
}

func TestCostService_TotalCost_NoRows_ReturnsZero(t *testing.T) {
	executor := &mockQueryExecutor{rowData: []any{float64(0), uint64(0)}}
	log := &testLogger{t: t}
	svc := service.NewCostService(executor, nil, nil, log)

	total, tokens, err := svc.TotalCost(context.Background(), domain.TimeWindow{})
	if err != nil {
		t.Fatalf("TotalCost should not error: %v", err)
	}
	if total != 0 || tokens != 0 {
		t.Errorf("TotalCost with no data should return 0, got total=%.2f tokens=%d", total, tokens)
	}
}

func TestCostService_TotalCost_WithData_ReturnsCorrectValues(t *testing.T) {
	executor := &mockQueryExecutor{rowData: []any{42.5, uint64(1000)}}
	log := &testLogger{t: t}
	svc := service.NewCostService(executor, nil, nil, log)

	total, tokens, err := svc.TotalCost(context.Background(), domain.TimeWindow{})
	if err != nil {
		t.Fatalf("TotalCost should not error: %v", err)
	}
	if total != 42.5 {
		t.Errorf("TotalCost total = %.2f, want 42.5", total)
	}
	if tokens != 1000 {
		t.Errorf("TotalCost tokens = %d, want 1000", tokens)
	}
}

func TestCostService_Breakdown_WithData_AggregatesCorrectly(t *testing.T) {
	// Simulate: 2 rows for user dimension
	rowsData := [][]any{
		{"user-alice", float64(10.5), uint64(500), uint64(3)},
		{"user-bob", float64(5.25), uint64(250), uint64(2)},
	}
	executor := newMockQueryExecutor(rowsData)
	log := &testLogger{t: t}
	svc := service.NewCostService(executor, nil, nil, log)

	results, err := svc.Breakdown(context.Background(), domain.TimeWindow{}, []domain.CostDimension{domain.DimensionUser}, 10)
	if err != nil {
		t.Fatalf("Breakdown should not error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	bd := results[0]
	if len(bd.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(bd.Items))
	}
	if bd.TotalUSD != 15.75 {
		t.Errorf("TotalUSD = %.2f, want 15.75", bd.TotalUSD)
	}
	if bd.TotalTokens != 750 {
		t.Errorf("TotalTokens = %d, want 750", bd.TotalTokens)
	}

	// Verify ranking
	for i, item := range bd.Items {
		if item.Rank != i+1 {
			t.Errorf("item %d rank = %d, want %d", i, item.Rank, i+1)
		}
	}
}

func TestCostService_Breakdown_DefaultDimensions(t *testing.T) {
	// Verifies that Breakdown with nil dimensions uses AllCostDimensions()
	executor := newMockQueryExecutor(nil)
	log := &testLogger{t: t}
	svc := service.NewCostService(executor, nil, nil, log)

	results, err := svc.Breakdown(context.Background(), domain.TimeWindow{}, nil, 0)
	if err != nil {
		t.Fatalf("Breakdown should not error: %v", err)
	}
	expectedDims := len(domain.AllCostDimensions())
	if len(results) != expectedDims {
		t.Errorf("expected %d results for default dimensions, got %d", expectedDims, len(results))
	}
}

func TestCostService_Breakdown_UnknownDimension_ReturnsError(t *testing.T) {
	executor := newMockQueryExecutor(nil)
	log := &testLogger{t: t}
	svc := service.NewCostService(executor, nil, nil, log)

	// Use a made-up dimension via dimensionConfig directly (simulating SQL error)
	_, err := svc.Breakdown(context.Background(), domain.TimeWindow{}, []domain.CostDimension{"unknown_dim"}, 10)
	if err == nil {
		t.Error("Breakdown with unknown dimension should return error")
	}
}

func TestCostService_Timeline_WithData_ReturnsPoints(t *testing.T) {
	rowsData := [][]any{
		{time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), float64(10.0), uint64(500), uint64(5)},
		{time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC), float64(15.0), uint64(750), uint64(8)},
	}
	executor := newMockQueryExecutor(rowsData)
	log := &testLogger{t: t}
	svc := service.NewCostService(executor, nil, nil, log)

	points, err := svc.Timeline(context.Background(), domain.TimeWindow{}, "hour")
	if err != nil {
		t.Fatalf("Timeline should not error: %v", err)
	}
	if len(points) != 2 {
		t.Errorf("expected 2 points, got %d", len(points))
	}
	if points[0].CostUSD != 10.0 || points[0].CallCount != 5 {
		t.Errorf("point[0] values incorrect")
	}
}

func TestCostService_Timeline_DefaultGranularity(t *testing.T) {
	executor := newMockQueryExecutor(nil)
	log := &testLogger{t: t}
	svc := service.NewCostService(executor, nil, nil, log)

	// Empty granularity should default to "hour"
	_, err := svc.Timeline(context.Background(), domain.TimeWindow{}, "")
	if err != nil {
		t.Fatalf("Timeline should not error with default granularity: %v", err)
	}
}

// 模块测试：Container.IngestSpans 触发评估采样 + Shutdown drain
package module_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
)

type memSpanRepo struct {
	n atomic.Int64
}

func (m *memSpanRepo) Insert(ctx context.Context, span *domain.Span) error {
	return m.BatchInsert(ctx, []*domain.Span{span})
}
func (m *memSpanRepo) BatchInsert(ctx context.Context, spans []*domain.Span) error {
	m.n.Add(int64(len(spans)))
	return nil
}
func (m *memSpanRepo) GetByID(ctx context.Context, id string) (*domain.Span, error) {
	return nil, nil
}
func (m *memSpanRepo) GetByTraceID(ctx context.Context, traceID string) ([]*domain.Span, error) {
	return nil, nil
}
func (m *memSpanRepo) ListBySession(ctx context.Context, sessionID string, opts domain.ListOptions) ([]*domain.Span, error) {
	return nil, nil
}
func (m *memSpanRepo) ListByUser(ctx context.Context, userID string, opts domain.ListOptions) ([]*domain.Span, error) {
	return nil, nil
}
func (m *memSpanRepo) ListByAgent(ctx context.Context, agentName string, opts domain.ListOptions) ([]*domain.Span, error) {
	return nil, nil
}
func (m *memSpanRepo) ListAllInWindow(ctx context.Context, opts domain.ListOptions) ([]*domain.Span, error) {
	return nil, nil
}
func (m *memSpanRepo) GetTraceTree(ctx context.Context, traceID string) (*domain.TraceTree, error) {
	return nil, nil
}

func TestContainerIngestQueuesSpans(t *testing.T) {
	repo := &memSpanRepo{}
	spanSvc := service.NewSpanService(repo, nil, logger.NewNop())
	c := &service.Container{SpanService: spanSvc}

	now := time.Now().UTC()
	spans := []*domain.Span{{
		ID: "s1", TraceID: "t1", SessionID: "sess", Type: domain.SpanTypeTool,
		StartTime: now, Name: "tool",
	}}
	if err := c.IngestSpans(context.Background(), spans); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	c.Shutdown(context.Background())
	// 给 worker 一点时间 flush
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if repo.n.Load() >= 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Shutdown 应 drain；若仍为 0 则失败
	if repo.n.Load() < 1 {
		t.Fatalf("expected spans flushed after shutdown, got %d", repo.n.Load())
	}
}

func TestContainerIngestNilSpanService(t *testing.T) {
	c := &service.Container{}
	if err := c.IngestSpans(context.Background(), nil); err != nil {
		t.Fatalf("nil service should no-op: %v", err)
	}
}

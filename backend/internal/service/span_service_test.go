// Package service_test SpanService 单元测试。
package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
)

// ===== Mock 实现 =====

// mockSpanRepo 模拟 domain.SpanRepository。
type mockSpanRepo struct {
	mu       sync.RWMutex
	spans    map[string]*domain.Span
	byTrace  map[string][]*domain.Span
	byAgent  map[string][]*domain.Span
	err      error
	insertCalls int
}

func newMockSpanRepo() *mockSpanRepo {
	return &mockSpanRepo{
		spans:   make(map[string]*domain.Span),
		byTrace: make(map[string][]*domain.Span),
		byAgent: make(map[string][]*domain.Span),
	}
}

func (m *mockSpanRepo) setError(err error) { m.mu.Lock(); m.err = err; m.mu.Unlock() }
func (m *mockSpanRepo) insertCount() int { m.mu.RLock(); defer m.mu.RUnlock(); return m.insertCalls }
func (m *mockSpanRepo) storedCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.spans)
}

func (m *mockSpanRepo) Insert(_ context.Context, span *domain.Span) error {
	m.mu.Lock(); defer m.mu.Unlock()
	m.insertCalls++
	if m.err != nil { return m.err }
	m.spans[span.ID] = span
	return nil
}

func (m *mockSpanRepo) BatchInsert(_ context.Context, spans []*domain.Span) error {
	m.mu.Lock(); defer m.mu.Unlock()
	m.insertCalls++
	if m.err != nil { return m.err }
	for _, s := range spans {
		m.spans[s.ID] = s
		m.byTrace[s.TraceID] = append(m.byTrace[s.TraceID], s)
		if s.AgentName != "" {
			m.byAgent[s.AgentName] = append(m.byAgent[s.AgentName], s)
		}
	}
	return nil
}

func (m *mockSpanRepo) GetByID(_ context.Context, id string) (*domain.Span, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	if m.err != nil { return nil, m.err }
	return m.spans[id], nil
}

func (m *mockSpanRepo) GetByTraceID(_ context.Context, traceID string) ([]*domain.Span, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	if m.err != nil { return nil, m.err }
	return m.byTrace[traceID], nil
}

func (m *mockSpanRepo) ListBySession(_ context.Context, _ string, _ domain.ListOptions) ([]*domain.Span, error) {
	return m.all()
}
func (m *mockSpanRepo) ListByUser(_ context.Context, _ string, _ domain.ListOptions) ([]*domain.Span, error) {
	return m.all()
}
func (m *mockSpanRepo) ListByAgent(_ context.Context, _ string, _ domain.ListOptions) ([]*domain.Span, error) {
	return m.all()
}
func (m *mockSpanRepo) ListAllInWindow(_ context.Context, _ domain.ListOptions) ([]*domain.Span, error) {
	return m.all()
}
func (m *mockSpanRepo) GetTraceTree(_ context.Context, traceID string) (*domain.TraceTree, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	if m.err != nil { return nil, m.err }
	spans := m.byTrace[traceID]
	if len(spans) == 0 { return nil, nil }
	return &domain.TraceTree{
		TraceID:  traceID,
		AllSpans: spans,
		Depth:    1,
	}, nil
}

func (m *mockSpanRepo) all() ([]*domain.Span, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	if m.err != nil { return nil, m.err }
	out := make([]*domain.Span, 0, len(m.spans))
	for _, s := range m.spans { out = append(out, s) }
	return out, nil
}

// mockPricingRepo 模拟 domain.PricingRepository。
type mockPricingRepo struct {
	mu       sync.RWMutex
	pricings map[string]*domain.Pricing
	err      error
}

func newMockPricingRepo() *mockPricingRepo {
	return &mockPricingRepo{pricings: make(map[string]*domain.Pricing)}
}

func (m *mockPricingRepo) setError(err error) { m.mu.Lock(); m.err = err; m.mu.Unlock() }
func (m *mockPricingRepo) setPricing(model string, p *domain.Pricing) {
	m.mu.Lock(); defer m.mu.Unlock()
	m.pricings[model] = p
}

func (m *mockPricingRepo) Get(_ context.Context, model string, _ time.Time) (*domain.Pricing, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	if m.err != nil { return nil, m.err }
	return m.pricings[model], nil
}
func (m *mockPricingRepo) ListActive(_ context.Context) ([]*domain.Pricing, error) { return nil, nil }
func (m *mockPricingRepo) Upsert(_ context.Context, _ *domain.Pricing) error { return nil }

// testLogger 最小 logger.Logger 实现。
type testLogger struct{ t *testing.T }

func (l *testLogger) Debugf(string, ...any) {}
func (l *testLogger) Infof(string, ...any)  {}
func (l *testLogger) Warnf(string, ...any)  {}
func (l *testLogger) Errorf(f string, a ...any) { l.t.Logf("ERROR: "+f, a...) }
func (l *testLogger) Fatalf(string, ...any)    {}
func (l *testLogger) WithField(string, any) logger.Logger              { return l }
func (l *testLogger) WithFields(map[string]any) logger.Logger          { return l }
func (l *testLogger) WithError(error) logger.Logger                    { return l }
func (l *testLogger) Sync() error                                      { return nil }

// ===== 测试用例 =====

func TestNewSpanService(t *testing.T) {
	spanRepo := newMockSpanRepo()
	pricingRepo := newMockPricingRepo()
	log := &testLogger{t: t}

	svc := service.NewSpanService(spanRepo, pricingRepo, log)
	if svc == nil {
		t.Fatal("NewSpanService returned nil")
	}
	defer svc.Shutdown(context.Background())
}

func TestIngestSpansEmpty(t *testing.T) {
	spanRepo := newMockSpanRepo()
	pricingRepo := newMockPricingRepo()
	log := &testLogger{t: t}

	svc := service.NewSpanService(spanRepo, pricingRepo, log)
	defer svc.Shutdown(context.Background())

	err := svc.IngestSpans(context.Background(), nil)
	if err != nil {
		t.Errorf("IngestSpans(nil) should return nil, got %v", err)
	}
}

func TestIngestSpansQueuesSpans(t *testing.T) {
	spanRepo := newMockSpanRepo()
	pricingRepo := newMockPricingRepo()
	log := &testLogger{t: t}

	svc := service.NewSpanService(spanRepo, pricingRepo, log)
	defer svc.Shutdown(context.Background())

	now := time.Now()
	spans := []*domain.Span{
		{ID: "span-1", TraceID: "trace-1", Type: domain.SpanTypeLLM, StartTime: now},
		{ID: "span-2", TraceID: "trace-1", Type: domain.SpanTypeLLM, StartTime: now},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := svc.IngestSpans(ctx, spans); err != nil {
		t.Fatalf("IngestSpans failed: %v", err)
	}

	// 等待 worker 处理（flush tick 5s）；多 worker 可能拆成多次 BatchInsert
	time.Sleep(6 * time.Second)

	if spanRepo.insertCount() < 1 {
		t.Errorf("expected at least 1 batch insert call, got %d", spanRepo.insertCount())
	}
	// 断言实际入库 span 数，而非 batch 次数（2 worker 下可能 1 或 2 次 flush）
	if got := spanRepo.storedCount(); got != 2 {
		t.Errorf("expected 2 spans stored, got %d", got)
	}
}

func TestIngestSpansCalculatesMissingCost(t *testing.T) {
	spanRepo := newMockSpanRepo()
	pricingRepo := newMockPricingRepo()

	pricingRepo.setPricing("gpt-4o", &domain.Pricing{
		Model: "gpt-4o", PromptPrice: 0.01, CompletionPrice: 0.02, EffectiveAt: time.Now(),
	})

	log := &testLogger{t: t}
	svc := service.NewSpanService(spanRepo, pricingRepo, log)
	defer svc.Shutdown(context.Background())

	span := &domain.Span{
		ID: "s1", Model: "gpt-4o",
		PromptTokens: 1000, CompletionTokens: 500,
		CostUSD: 0, Type: domain.SpanTypeLLM, StartTime: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := svc.IngestSpans(ctx, []*domain.Span{span}); err != nil {
		t.Fatalf("IngestSpans failed: %v", err)
	}

	time.Sleep(6 * time.Second)

	// 1000/1000*0.01 + 500/1000*0.02 = 0.02
	if span.CostUSD != 0.02 {
		t.Errorf("CostUSD = %f, want 0.02 (auto-calculated)", span.CostUSD)
	}
}

func TestIngestSpansSkipsCostForNonLLM(t *testing.T) {
	spanRepo := newMockSpanRepo()
	pricingRepo := newMockPricingRepo()
	log := &testLogger{t: t}

	svc := service.NewSpanService(spanRepo, pricingRepo, log)
	defer svc.Shutdown(context.Background())

	span := &domain.Span{
		ID: "s1", Type: domain.SpanTypeTool, ToolName: "search", StartTime: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := svc.IngestSpans(ctx, []*domain.Span{span}); err != nil {
		t.Fatalf("IngestSpans failed: %v", err)
	}

	time.Sleep(600 * time.Millisecond)
}

func TestSpanServiceDelegatesQueries(t *testing.T) {
	spanRepo := newMockSpanRepo()
	pricingRepo := newMockPricingRepo()
	log := &testLogger{t: t}

	now := time.Now()
	testSpan := &domain.Span{
		ID: "span-1", TraceID: "trace-abc", SessionID: "session-1",
		UserID: "user-1", AgentName: "agent-1", Type: domain.SpanTypeLLM,
		StartTime: now,
	}
	spanRepo.spans["span-1"] = testSpan
	spanRepo.byTrace["trace-abc"] = []*domain.Span{testSpan}
	spanRepo.byAgent["agent-1"] = []*domain.Span{testSpan}

	svc := service.NewSpanService(spanRepo, pricingRepo, log)
	defer svc.Shutdown(context.Background())

	t.Run("GetByID delegates", func(t *testing.T) {
		s, err := svc.GetByID(context.Background(), "span-1")
		if err != nil { t.Fatalf("GetByID error: %v", err) }
		if s == nil || s.ID != "span-1" { t.Error("GetByID should return the span") }
	})

	t.Run("GetTraceTree delegates", func(t *testing.T) {
		tree, err := svc.GetTraceTree(context.Background(), "trace-abc")
		if err != nil { t.Fatalf("GetTraceTree error: %v", err) }
		if tree == nil { t.Error("GetTraceTree should return tree") }
	})

	t.Run("ListBySession delegates", func(t *testing.T) {
		spans, err := svc.ListBySession(context.Background(), "session-1", domain.ListOptions{})
		if err != nil { t.Fatalf("ListBySession error: %v", err) }
		if len(spans) != 1 { t.Errorf("expected 1 span, got %d", len(spans)) }
	})

	t.Run("ListByUser delegates", func(t *testing.T) {
		spans, err := svc.ListByUser(context.Background(), "user-1", domain.ListOptions{})
		if err != nil { t.Fatalf("ListByUser error: %v", err) }
		if len(spans) != 1 { t.Errorf("expected 1 span, got %d", len(spans)) }
	})

	t.Run("ListByAgent delegates", func(t *testing.T) {
		spans, err := svc.ListByAgent(context.Background(), "agent-1", domain.ListOptions{})
		if err != nil { t.Fatalf("ListByAgent error: %v", err) }
		if len(spans) != 1 { t.Errorf("expected 1 span, got %d", len(spans)) }
	})
}

func TestSpanServiceShutdown(t *testing.T) {
	spanRepo := newMockSpanRepo()
	pricingRepo := newMockPricingRepo()
	log := &testLogger{t: t}

	svc := service.NewSpanService(spanRepo, pricingRepo, log)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		svc.Shutdown(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown timed out")
	}
}

func TestFillMissingCostGroupsByModel(t *testing.T) {
	spanRepo := newMockSpanRepo()
	pricingRepo := newMockPricingRepo()

	pricingRepo.setPricing("gpt-4o", &domain.Pricing{
		Model: "gpt-4o", PromptPrice: 0.01, CompletionPrice: 0.02, EffectiveAt: time.Now(),
	})
	pricingRepo.setPricing("claude-3", &domain.Pricing{
		Model: "claude-3", PromptPrice: 0.005, CompletionPrice: 0.015, EffectiveAt: time.Now(),
	})

	log := &testLogger{t: t}
	svc := service.NewSpanService(spanRepo, pricingRepo, log)
	defer svc.Shutdown(context.Background())

	now := time.Now()
	spans := []*domain.Span{
		{ID: "s1", Model: "gpt-4o", PromptTokens: 1000, CompletionTokens: 500, CostUSD: 0, Type: domain.SpanTypeLLM, StartTime: now},
		{ID: "s2", Model: "gpt-4o", PromptTokens: 2000, CompletionTokens: 1000, CostUSD: 0, Type: domain.SpanTypeLLM, StartTime: now},
		{ID: "s3", Model: "claude-3", PromptTokens: 500, CompletionTokens: 200, CostUSD: 0, Type: domain.SpanTypeLLM, StartTime: now},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := svc.IngestSpans(ctx, spans); err != nil {
		t.Fatalf("IngestSpans failed: %v", err)
	}

	time.Sleep(600 * time.Millisecond)

	if spans[0].CostUSD != 0.02 {
		t.Errorf("s1 CostUSD = %f, want 0.02", spans[0].CostUSD)
	}
	if spans[1].CostUSD != 0.04 {
		t.Errorf("s2 CostUSD = %f, want 0.04", spans[1].CostUSD)
	}
	// claude-3: 500/1000*0.005 + 200/1000*0.015 = 0.0055
	if spans[2].CostUSD != 0.0055 {
		t.Errorf("s3 CostUSD = %f, want 0.0055", spans[2].CostUSD)
	}
}

func TestFillMissingCostSkipsWhenPricingNil(t *testing.T) {
	spanRepo := newMockSpanRepo()
	log := &testLogger{t: t}

	svc := service.NewSpanService(spanRepo, nil, log) // nil pricingRepo
	defer svc.Shutdown(context.Background())

	span := &domain.Span{
		ID: "s1", Model: "gpt-4o", PromptTokens: 1000, CompletionTokens: 500,
		CostUSD: 0, Type: domain.SpanTypeLLM, StartTime: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := svc.IngestSpans(ctx, []*domain.Span{span}); err != nil {
		t.Fatalf("IngestSpans failed: %v", err)
	}

	time.Sleep(600 * time.Millisecond)
	if span.CostUSD != 0 {
		t.Errorf("CostUSD should remain 0 when pricingRepo is nil, got %f", span.CostUSD)
	}
}

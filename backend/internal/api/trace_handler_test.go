// Package api API handler 单元测试。
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// ===== 测试基础设施 =====

// mockSpanRepoForAPI 最小 SpanRepository 模拟。
type mockSpanRepoForAPI struct {
	spans   map[string]*domain.Span
	byTrace map[string][]*domain.Span
}

func newMockSpanRepoForAPI() *mockSpanRepoForAPI {
	return &mockSpanRepoForAPI{
		spans:   make(map[string]*domain.Span),
		byTrace: make(map[string][]*domain.Span),
	}
}

func (m *mockSpanRepoForAPI) Insert(context.Context, *domain.Span) error { return nil }
func (m *mockSpanRepoForAPI) BatchInsert(context.Context, []*domain.Span) error { return nil }
func (m *mockSpanRepoForAPI) GetByID(_ context.Context, id string) (*domain.Span, error) {
	return m.spans[id], nil
}
func (m *mockSpanRepoForAPI) GetByTraceID(_ context.Context, traceID string) ([]*domain.Span, error) {
	return m.byTrace[traceID], nil
}
func (m *mockSpanRepoForAPI) ListBySession(context.Context, string, domain.ListOptions) ([]*domain.Span, error) {
	return nil, nil
}
func (m *mockSpanRepoForAPI) ListByUser(context.Context, string, domain.ListOptions) ([]*domain.Span, error) {
	return nil, nil
}
func (m *mockSpanRepoForAPI) ListByAgent(context.Context, string, domain.ListOptions) ([]*domain.Span, error) {
	return nil, nil
}
func (m *mockSpanRepoForAPI) ListAllInWindow(context.Context, domain.ListOptions) ([]*domain.Span, error) {
	return nil, nil
}
func (m *mockSpanRepoForAPI) GetTraceTree(_ context.Context, traceID string) (*domain.TraceTree, error) {
	spans := m.byTrace[traceID]
	if len(spans) == 0 { return nil, nil }
	return &domain.TraceTree{TraceID: traceID, AllSpans: spans, Depth: 1}, nil
}

// mockPricingRepoForAPI 最小 PricingRepository 模拟。
type mockPricingRepoForAPI struct{}

func newMockPricingRepoForAPI() *mockPricingRepoForAPI { return &mockPricingRepoForAPI{} }
func (m *mockPricingRepoForAPI) Get(context.Context, string, time.Time) (*domain.Pricing, error) { return nil, nil }
func (m *mockPricingRepoForAPI) ListActive(context.Context) ([]*domain.Pricing, error) { return nil, nil }
func (m *mockPricingRepoForAPI) Upsert(context.Context, *domain.Pricing) error { return nil }

// testServer 封装 gin 路由用于测试。
type testServer struct {
	router *gin.Engine
}

func newTestServer(services *service.Container) *testServer {
	r := gin.New()
	r.Use(RecoveryMiddleware(nilLogger{}))
	r.Use(RequestIDMiddleware())

	traceHandler := NewTraceHandler(services, nilLogger{})
	traceGroup := r.Group("/api/v1/traces")
	traceGroup.GET("/:trace_id", traceHandler.GetTraceTree)
	traceGroup.GET("/sessions/:session_id/spans", traceHandler.ListBySession)
	traceGroup.GET("/users/:user_id/spans", traceHandler.ListByUser)
	traceGroup.GET("/agents/:agent_name/spans", traceHandler.ListByAgent)

	return &testServer{router: r}
}

func (s *testServer) do(req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

// newListTestServer 创建带有 ListBySession 路由的测试服务器。
// 预置 trace 数据让 handler 走到 parseListOptions。
func newListTestServer(services *service.Container) *testServer {
	r := gin.New()
	r.Use(RecoveryMiddleware(nilLogger{}))
	r.Use(RequestIDMiddleware())

	traceHandler := NewTraceHandler(services, nilLogger{})
	r.GET("/api/v1/sessions/:session_id/spans", traceHandler.ListBySession)

	return &testServer{router: r}
}

// nilLogger 满足 logger.Logger 接口的空实现。
type nilLogger struct{}

func (nilLogger) Debugf(string, ...any) {}
func (nilLogger) Infof(string, ...any)  {}
func (nilLogger) Warnf(string, ...any)  {}
func (nilLogger) Errorf(string, ...any) {}
func (nilLogger) Fatalf(string, ...any) {}
func (nilLogger) WithField(string, any) logger.Logger              { return nilLogger{} }
func (nilLogger) WithFields(map[string]any) logger.Logger          { return nilLogger{} }
func (nilLogger) WithError(error) logger.Logger                    { return nilLogger{} }
func (nilLogger) Sync() error                                      { return nil }

func boolPtr(b bool) *bool { return &b }

// ===== TraceHandler 测试 =====

func TestGetTraceTreeInvalidTraceID(t *testing.T) {
	spanRepo := newMockSpanRepoForAPI()
	pricingRepo := newMockPricingRepoForAPI()

	services := &service.Container{
		SpanRepo:     spanRepo,
		PricingRepo:  pricingRepo,
		SpanService:  service.NewSpanService(spanRepo, pricingRepo, nilLogger{}),
	}

	srv := newTestServer(services)

	tests := []struct {
		name     string
		traceID  string
		wantCode int
	}{
		{"empty trace_id path", "/api/v1/traces/", http.StatusNotFound},
		{"too short", "/api/v1/traces/abc", http.StatusBadRequest},
		{"too long", "/api/v1/traces/" + strings.Repeat("a", 33), http.StatusBadRequest},
		{"non-hex chars", "/api/v1/traces/gggggggggggggggggggggggggggggggg", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.traceID, nil)
			w := srv.do(req)
			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d (body: %s)", w.Code, tt.wantCode, w.Body.String())
			}
		})
	}
}

func TestGetTraceTreeValidFormat(t *testing.T) {
	spanRepo := newMockSpanRepoForAPI()
	pricingRepo := newMockPricingRepoForAPI()
	services := &service.Container{
		SpanRepo:    spanRepo,
		PricingRepo: pricingRepo,
		SpanService: service.NewSpanService(spanRepo, pricingRepo, nilLogger{}),
	}

	srv := newTestServer(services)

	// 合法的 32 位十六进制 trace ID
	req := httptest.NewRequest(http.MethodGet, "/api/v1/traces/0123456789abcdef0123456789abcdef", nil)
	w := srv.do(req)

	// 应该返回 200 或 404（取决于 mock 中是否有数据），但不应该返回 400
	if w.Code == http.StatusBadRequest {
		t.Errorf("valid trace_id should not return 400, got %d", w.Code)
	}
}

func TestGetTraceTreeNotFound(t *testing.T) {
	spanRepo := newMockSpanRepoForAPI()
	pricingRepo := newMockPricingRepoForAPI()
	services := &service.Container{
		SpanRepo:    spanRepo,
		PricingRepo: pricingRepo,
		SpanService: service.NewSpanService(spanRepo, pricingRepo, nilLogger{}),
	}

	srv := newTestServer(services)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/traces/0123456789abcdef0123456789abcdef", nil)
	w := srv.do(req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing trace, got %d", w.Code)
	}
}

// ===== parseListOptions 边界测试（通过 ListBySession 端点） =====

func TestParseListOptionsLimits(t *testing.T) {
	spanRepo := newMockSpanRepoForAPI()
	spanRepo.spans["0123456789abcdef0123456789abcdef"] = &domain.Span{ID: "0123456789abcdef0123456789abcdef"}
	spanRepo.byTrace["0123456789abcdef0123456789abcdef"] = []*domain.Span{{ID: "0123456789abcdef0123456789abcdef"}}
	pricingRepo := newMockPricingRepoForAPI()
	services := &service.Container{
		SpanRepo:    spanRepo,
		PricingRepo: pricingRepo,
		SpanService: service.NewSpanService(spanRepo, pricingRepo, nilLogger{}),
	}

	r := gin.New()
	r.Use(RecoveryMiddleware(nilLogger{}))
	r.Use(RequestIDMiddleware())
	traceHandler := NewTraceHandler(services, nilLogger{})
	r.GET("/api/v1/sessions/:session_id/spans", traceHandler.ListBySession)

	t.Run("limit capped at 1000", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1/spans?limit=9999", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusBadRequest {
			t.Errorf("limit=9999 should be capped, not rejected: %s", w.Body.String())
		}
	})

	t.Run("negative limit rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1/spans?limit=-1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("limit=-1 should return 400, got %d", w.Code)
		}
	})
}

func TestParseListOptionsOrderBy(t *testing.T) {
	spanRepo := newMockSpanRepoForAPI()
	spanRepo.spans["0123456789abcdef0123456789abcdef"] = &domain.Span{ID: "0123456789abcdef0123456789abcdef"}
	spanRepo.byTrace["0123456789abcdef0123456789abcdef"] = []*domain.Span{{ID: "0123456789abcdef0123456789abcdef"}}
	pricingRepo := newMockPricingRepoForAPI()
	services := &service.Container{
		SpanRepo:    spanRepo,
		PricingRepo: pricingRepo,
		SpanService: service.NewSpanService(spanRepo, pricingRepo, nilLogger{}),
	}

	r := gin.New()
	r.Use(RecoveryMiddleware(nilLogger{}))
	r.Use(RequestIDMiddleware())
	traceHandler := NewTraceHandler(services, nilLogger{})
	r.GET("/api/v1/sessions/:session_id/spans", traceHandler.ListBySession)

	t.Run("invalid order_by rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1/spans?order_by=evil_column", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("invalid order_by should return 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("valid order_by accepted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1/spans?order_by=timestamp", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusBadRequest {
			t.Errorf("valid order_by should not return 400: %s", w.Body.String())
		}
	})
}

func TestParseListOptionsTimeRange(t *testing.T) {
	spanRepo := newMockSpanRepoForAPI()
	spanRepo.spans["0123456789abcdef0123456789abcdef"] = &domain.Span{ID: "0123456789abcdef0123456789abcdef"}
	spanRepo.byTrace["0123456789abcdef0123456789abcdef"] = []*domain.Span{{ID: "0123456789abcdef0123456789abcdef"}}
	pricingRepo := newMockPricingRepoForAPI()
	services := &service.Container{
		SpanRepo:    spanRepo,
		PricingRepo: pricingRepo,
		SpanService: service.NewSpanService(spanRepo, pricingRepo, nilLogger{}),
	}

	r := gin.New()
	r.Use(RecoveryMiddleware(nilLogger{}))
	r.Use(RequestIDMiddleware())
	traceHandler := NewTraceHandler(services, nilLogger{})
	r.GET("/api/v1/sessions/:session_id/spans", traceHandler.ListBySession)

	t.Run("from after to rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/sessions/session-1/spans?from=2025-06-01T00:00:00Z&to=2025-01-01T00:00:00Z",
			nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("from > to should return 400, got %d (body: %s)", w.Code, w.Body.String())
		}
	})

	t.Run("invalid time format rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/sessions/session-1/spans?from=not-a-date",
			nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("invalid time format should return 400, got %d (body: %s)", w.Code, w.Body.String())
		}
	})
}

// ===== 辅助 =====

func init() {
	// 确保 gin 在测试模式下
	gin.SetMode(gin.TestMode)
}

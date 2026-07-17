// Package collector - OTLP HTTP/gRPC handler 行为测试。
package collector

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
)

func TestWriteSuccessPartialSuccess(t *testing.T) {
	h := &HTTPHandler{logger: logger.NewNop()}
	rec := httptest.NewRecorder()
	h.writeSuccess(rec, 3, "ingest failed: boom")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp collectorpb.ExportTraceServiceResponse
	if err := proto.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.PartialSuccess == nil {
		t.Fatal("expected PartialSuccess")
	}
	if resp.PartialSuccess.RejectedSpans != 3 {
		t.Errorf("RejectedSpans = %d, want 3", resp.PartialSuccess.RejectedSpans)
	}
	if resp.PartialSuccess.ErrorMessage == "" {
		t.Error("ErrorMessage empty")
	}
}

func TestWriteSuccessNoPartial(t *testing.T) {
	h := &HTTPHandler{logger: logger.NewNop()}
	rec := httptest.NewRecorder()
	h.writeSuccess(rec, 2, "")

	var resp collectorpb.ExportTraceServiceResponse
	if err := proto.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.PartialSuccess != nil {
		t.Errorf("unexpected PartialSuccess: %+v", resp.PartialSuccess)
	}
}

func TestHTTPHandlerEmptySpansOK(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.OTLPRequireKey = boolPtr(false)
	cfg.OTLP.MaxBodySize = 1 << 20

	h := NewHTTPHandler(cfg, &service.Container{}, logger.NewNop())
	body, _ := proto.Marshal(&collectorpb.ExportTraceServiceRequest{})
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTPHandlerIngestFailurePartialSuccess(t *testing.T) {
	// 使用已 Shutdown 的 SpanService 使 IngestSpans 返回 service closed
	cfg := &config.Config{}
	cfg.Auth.OTLPRequireKey = boolPtr(false)
	cfg.OTLP.MaxBodySize = 1 << 20

	spanSvc := service.NewSpanService(&noopSpanRepo{}, nil, logger.NewNop())
	spanSvc.Shutdown(context.Background())
	svc := &service.Container{SpanService: spanSvc}

	h := NewHTTPHandler(cfg, svc, logger.NewNop())
	reqBody, err := proto.Marshal(newTestRequest(mkSpan("t", "agent")))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp collectorpb.ExportTraceServiceResponse
	raw, _ := io.ReadAll(rec.Body)
	if err := proto.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.PartialSuccess == nil {
		t.Fatal("expected PartialSuccess on ingest failure")
	}
	if resp.PartialSuccess.RejectedSpans < 1 {
		t.Errorf("RejectedSpans = %d, want >= 1", resp.PartialSuccess.RejectedSpans)
	}
	if resp.PartialSuccess.ErrorMessage == "" {
		t.Error("ErrorMessage should be set")
	}
}

func boolPtr(b bool) *bool { return &b }

// noopSpanRepo 满足 domain.SpanRepository 最小接口。
type noopSpanRepo struct{}

func (noopSpanRepo) Insert(ctx context.Context, span *domain.Span) error { return nil }
func (noopSpanRepo) BatchInsert(ctx context.Context, spans []*domain.Span) error {
	return nil
}
func (noopSpanRepo) GetByID(ctx context.Context, id string) (*domain.Span, error) {
	return nil, nil
}
func (noopSpanRepo) GetByTraceID(ctx context.Context, traceID string) ([]*domain.Span, error) {
	return nil, nil
}
func (noopSpanRepo) GetTraceTree(ctx context.Context, traceID string) (*domain.TraceTree, error) {
	return nil, nil
}
func (noopSpanRepo) ListBySession(ctx context.Context, sessionID string, opts domain.ListOptions) ([]*domain.Span, error) {
	return nil, nil
}
func (noopSpanRepo) ListByUser(ctx context.Context, userID string, opts domain.ListOptions) ([]*domain.Span, error) {
	return nil, nil
}
func (noopSpanRepo) ListByAgent(ctx context.Context, agentName string, opts domain.ListOptions) ([]*domain.Span, error) {
	return nil, nil
}
func (noopSpanRepo) ListAllInWindow(ctx context.Context, opts domain.ListOptions) ([]*domain.Span, error) {
	return nil, nil
}

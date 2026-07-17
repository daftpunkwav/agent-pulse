// 功能测试：OTLP HTTP 入队失败返回 PartialSuccess
package functional_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/agentpulse/backend/internal/collector"
	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
)

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

func boolPtr(b bool) *bool { return &b }

func mkExportReq() *collectorpb.ExportTraceServiceRequest {
	now := uint64(1_700_000_000_000_000_000)
	// OTLP 规范：TraceId 16 字节，SpanId 8 字节
	traceID := make([]byte, 16)
	spanID := make([]byte, 8)
	for i := range traceID {
		traceID[i] = byte(i + 1)
	}
	for i := range spanID {
		spanID[i] = byte(i + 10)
	}
	return &collectorpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{
				Attributes: []*commonpb.KeyValue{{
					Key: "service.name",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "t"}},
				}},
			},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Spans: []*tracepb.Span{{
					Name:              "agent-call",
					TraceId:           traceID,
					SpanId:            spanID,
					StartTimeUnixNano: now,
					EndTimeUnixNano:   now + 1e6,
					Attributes: []*commonpb.KeyValue{{
						Key:   "ap.span_type",
						Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "agent"}},
					}},
				}},
			}},
		}},
	}
}

func TestOTLPHTTPPartialSuccessWhenServiceClosed(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.OTLPRequireKey = boolPtr(false)
	cfg.OTLP.MaxBodySize = 1 << 20

	spanSvc := service.NewSpanService(noopSpanRepo{}, nil, logger.NewNop())
	spanSvc.Shutdown(context.Background())
	svc := &service.Container{SpanService: spanSvc}
	h := collector.NewHTTPHandler(cfg, svc, logger.NewNop())

	body, err := proto.Marshal(mkExportReq())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	raw, _ := io.ReadAll(rec.Body)
	var resp collectorpb.ExportTraceServiceResponse
	if err := proto.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.PartialSuccess == nil || resp.PartialSuccess.RejectedSpans < 1 {
		t.Fatalf("expected PartialSuccess with rejected spans, got %+v", resp.PartialSuccess)
	}
}

func TestOTLPHTTPEmptyOK(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.OTLPRequireKey = boolPtr(false)
	cfg.OTLP.MaxBodySize = 1 << 20
	h := collector.NewHTTPHandler(cfg, &service.Container{}, logger.NewNop())
	body, _ := proto.Marshal(&collectorpb.ExportTraceServiceRequest{})
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
}

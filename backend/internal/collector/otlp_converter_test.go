// Package collector - OTLP 转换器测试。
//
// 验证 ConvertOTLP / SpanFromOTLP / MapSemanticConventions 等共享转换函数。
package collector

import (
	"testing"
	"time"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/agentpulse/backend/internal/domain"
)

func strVal(s string) *commonpb.AnyValue {
	return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: s}}
}
func intVal(v int64) *commonpb.AnyValue {
	return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}
}
func floatVal(v float64) *commonpb.AnyValue {
	return &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: v}}
}
func kv(key string, val *commonpb.AnyValue) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: val}
}

// mkSpan 构造最小 OTLP Span（带基本属性）。
func mkSpan(name, spanType string) *tracepb.Span {
	now := time.Now()
	return &tracepb.Span{
		Name: name,
		TraceId: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		SpanId: []byte("1111111111111111"),
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano: uint64(now.Add(100 * time.Millisecond).UnixNano()),
		Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		Attributes: []*commonpb.KeyValue{
			kv("ap.span_type", strVal(spanType)),
		},
	}
}

// newTestRequest 构造最小 ExportTraceServiceRequest。
func newTestRequest(spans ...*tracepb.Span) *collectorpb.ExportTraceServiceRequest {
	return &collectorpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						kv("service.name", strVal("test-service")),
						kv("deployment.environment", strVal("test")),
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope:  &commonpb.InstrumentationScope{Name: "agentpulse"},
						Spans:  spans,
					},
				},
			},
		},
	}
}

func TestConvertOTLP_BasicConversion(t *testing.T) {
	span := mkSpan("llm-call", "llm")
	span.Attributes = append(span.Attributes,
		kv("ap.model", strVal("gpt-4o")),
		kv("ap.prompt_tokens", intVal(100)),
		kv("ap.completion_tokens", intVal(50)),
		kv("ap.user_id", strVal("user-1")),
		kv("ap.session_id", strVal("session-1")),
		kv("ap.agent_name", strVal("agent-1")),
	)
	req := newTestRequest(span)

	result := ConvertOTLP(req)
	if len(result) != 1 {
		t.Fatalf("expected 1 span, got %d", len(result))
	}

	s := result[0]
	if s.Type != domain.SpanTypeLLM {
		t.Errorf("span type = %s, want llm", s.Type)
	}
	if s.Model != "gpt-4o" {
		t.Errorf("model = %s, want gpt-4o", s.Model)
	}
	if s.PromptTokens != 100 {
		t.Errorf("prompt tokens = %d, want 100", s.PromptTokens)
	}
	if s.CompletionTokens != 50 {
		t.Errorf("completion tokens = %d, want 50", s.CompletionTokens)
	}
	if s.TotalTokens != 150 {
		t.Errorf("total tokens = %d, want 150", s.TotalTokens)
	}
	if s.ServiceName != "test-service" {
		t.Errorf("service name = %s, want test-service", s.ServiceName)
	}
	if s.Environment != "test" {
		t.Errorf("environment = %s, want test", s.Environment)
	}
	if s.UserID != "user-1" {
		t.Errorf("user_id = %s, want user-1", s.UserID)
	}
	if s.SessionID != "session-1" {
		t.Errorf("session_id = %s, want session-1", s.SessionID)
	}
	if s.AgentName != "agent-1" {
		t.Errorf("agent_name = %s, want agent-1", s.AgentName)
	}
}

func TestConvertOTLP_ErrorStatus(t *testing.T) {
	now := time.Now()
	span := &tracepb.Span{
		Name: "failed-llm",
		TraceId: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		SpanId: []byte("1111111111111111"),
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano: uint64(now.Add(100 * time.Millisecond).UnixNano()),
		Status: &tracepb.Status{
			Code: tracepb.Status_STATUS_CODE_ERROR,
			Message: "rate limit exceeded",
		},
		Attributes: []*commonpb.KeyValue{
			kv("ap.span_type", strVal("llm")),
		},
	}
	req := newTestRequest(span)

	result := ConvertOTLP(req)
	if len(result) != 1 {
		t.Fatalf("expected 1 span, got %d", len(result))
	}

	s := result[0]
	if s.Status != domain.SpanStatusError {
		t.Errorf("status = %s, want error", s.Status)
	}
	if s.ErrorMessage != "rate limit exceeded" {
		t.Errorf("error message = %s, want 'rate limit exceeded'", s.ErrorMessage)
	}
}

func TestConvertOTLP_MultipleSpans(t *testing.T) {
	now := time.Now()
	span1 := &tracepb.Span{
		Name: "span-1",
		TraceId: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		SpanId: []byte("1111111111111111"),
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano: uint64(now.Add(100 * time.Millisecond).UnixNano()),
		Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		Attributes: []*commonpb.KeyValue{
			kv("ap.span_type", strVal("agent")),
		},
	}
	span2 := &tracepb.Span{
		Name: "span-2",
		TraceId: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		SpanId: []byte("2222222222222222"),
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano: uint64(now.Add(200 * time.Millisecond).UnixNano()),
		Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		Attributes: []*commonpb.KeyValue{
			kv("ap.span_type", strVal("tool")),
			kv("ap.tool_name", strVal("search")),
		},
	}
	req := newTestRequest(span1, span2)

	result := ConvertOTLP(req)
	if len(result) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(result))
	}

	if result[0].Type != domain.SpanTypeAgent {
		t.Errorf("span[0] type = %s, want agent", result[0].Type)
	}
	if result[1].Type != domain.SpanTypeTool {
		t.Errorf("span[1] type = %s, want tool", result[1].Type)
	}
	if result[1].ToolName != "search" {
		t.Errorf("span[1] tool_name = %s, want search", result[1].ToolName)
	}
}

func TestConvertOTLP_MultipleResourceSpans(t *testing.T) {
	now := time.Now()
	span := &tracepb.Span{
		Name: "cross-service",
		TraceId: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		SpanId: []byte("ffffffffffffffff"),
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano: uint64(now.Add(50 * time.Millisecond).UnixNano()),
		Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		Attributes: []*commonpb.KeyValue{
			kv("ap.span_type", strVal("agent")),
		},
	}
	req := &collectorpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{kv("service.name", strVal("svc-a"))},
				},
				ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{span}}},
			},
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{kv("service.name", strVal("svc-b"))},
				},
				ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{span}}},
			},
		},
	}

	result := ConvertOTLP(req)
	if len(result) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(result))
	}
	if result[0].ServiceName != "svc-a" {
		t.Errorf("span[0] service = %s, want svc-a", result[0].ServiceName)
	}
	if result[1].ServiceName != "svc-b" {
		t.Errorf("span[1] service = %s, want svc-b", result[1].ServiceName)
	}
}

func TestConvertOTLP_EmptyRequest(t *testing.T) {
	req := newTestRequest()
	result := ConvertOTLP(req)
	if len(result) != 0 {
		t.Errorf("expected 0 spans for empty request, got %d", len(result))
	}
}

func TestConvertOTLP_MultipleScopeSpans(t *testing.T) {
	now := time.Now()
	span := &tracepb.Span{
		Name: "multi-scope",
		TraceId: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		SpanId: []byte("bbbbbbbbbbbbbbbb"),
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano: uint64(now.Add(50 * time.Millisecond).UnixNano()),
		Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		Attributes: []*commonpb.KeyValue{
			kv("ap.span_type", strVal("agent")),
		},
	}
	req := &collectorpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{kv("service.name", strVal("svc"))},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{Scope: &commonpb.InstrumentationScope{Name: "scope-a"}, Spans: []*tracepb.Span{span}},
					{Scope: &commonpb.InstrumentationScope{Name: "scope-b"}, Spans: []*tracepb.Span{span}},
				},
			},
		},
	}

	result := ConvertOTLP(req)
	if len(result) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(result))
	}
}

func TestConvertOTLP_SpanAttributes(t *testing.T) {
	now := time.Now()
	span := &tracepb.Span{
		Name: "with-attrs",
		TraceId: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		SpanId: []byte("cccccccccccccccc"),
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano: uint64(now.Add(100 * time.Millisecond).UnixNano()),
		Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		Attributes: []*commonpb.KeyValue{
			kv("ap.span_type", strVal("llm")),
			kv("ap.model", strVal("claude-3")),
			kv("ap.prompt_tokens", intVal(200)),
			kv("ap.completion_tokens", intVal(100)),
			kv("ap.cost_usd", floatVal(0.005)),
			kv("ap.finish_reason", strVal("stop")),
		},
	}
	req := newTestRequest(span)

	result := ConvertOTLP(req)
	if len(result) != 1 {
		t.Fatalf("expected 1 span, got %d", len(result))
	}

	s := result[0]
	if s.Model != "claude-3" {
		t.Errorf("model = %s, want claude-3", s.Model)
	}
	if s.PromptTokens != 200 {
		t.Errorf("prompt_tokens = %d, want 200", s.PromptTokens)
	}
	if s.CompletionTokens != 100 {
		t.Errorf("completion_tokens = %d, want 100", s.CompletionTokens)
	}
	if s.CostUSD != 0.005 {
		t.Errorf("cost_usd = %.4f, want 0.005", s.CostUSD)
	}
	if s.FinishReason != "stop" {
		t.Errorf("finish_reason = %s, want stop", s.FinishReason)
	}
}

func TestConvertOTLP_ResourceDefaults(t *testing.T) {
	now := time.Now()
	span := &tracepb.Span{
		Name: "default-attrs",
		TraceId: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		SpanId: []byte("dddddddddddddddd"),
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano: uint64(now.Add(50 * time.Millisecond).UnixNano()),
		Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		Attributes: []*commonpb.KeyValue{
			kv("ap.span_type", strVal("agent")),
		},
	}
	// 不提供 service.name 和 deployment.environment
	req := &collectorpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{},
				ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{span}}},
			},
		},
	}

	result := ConvertOTLP(req)
	if len(result) != 1 {
		t.Fatalf("expected 1 span, got %d", len(result))
	}

	s := result[0]
	if s.ServiceName != "unknown" {
		t.Errorf("service_name = %s, want 'unknown' (default)", s.ServiceName)
	}
	if s.Environment != "production" {
		t.Errorf("environment = %s, want 'production' (default)", s.Environment)
	}
}

func TestConvertOTLP_NilResource(t *testing.T) {
	now := time.Now()
	span := &tracepb.Span{
		Name: "nil-res",
		TraceId: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		SpanId: []byte("eeeeeeeeeeeeeeee"),
		StartTimeUnixNano: uint64(now.UnixNano()),
		EndTimeUnixNano: uint64(now.Add(50 * time.Millisecond).UnixNano()),
		Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		Attributes: []*commonpb.KeyValue{
			kv("ap.span_type", strVal("agent")),
		},
	}
	// ResourceSpans[0].Resource = nil
	req := &collectorpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: nil,
				ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{span}}},
			},
		},
	}

	result := ConvertOTLP(req)
	if len(result) != 1 {
		t.Fatalf("expected 1 span with nil resource, got %d", len(result))
	}
}

func TestMapSemanticConventions_GenAIFallback(t *testing.T) {
	attrs := map[string]any{
		"gen_ai.request.model": "gemini-pro",
		"gen_ai.usage.input_tokens": int64(100),
		"gen_ai.usage.output_tokens": int64(50),
	}

	spanType, model, prompt, completion, total, _, finishReason, _, _, _, _ :=
		MapSemanticConventions(attrs)

	if spanType != domain.SpanTypeAgent {
		t.Errorf("default span_type = %s, want agent", spanType)
	}
	if model != "gemini-pro" {
		t.Errorf("model = %s, want gemini-pro", model)
	}
	if prompt != 100 {
		t.Errorf("prompt_tokens = %d, want 100", prompt)
	}
	if completion != 50 {
		t.Errorf("completion_tokens = %d, want 50", completion)
	}
	if total != 150 {
		t.Errorf("total_tokens = %d, want 150", total)
	}
	if finishReason != "" {
		t.Errorf("finish_reason = %s, want empty", finishReason)
	}
}

func TestMapSemanticConventions_APFieldsOverride(t *testing.T) {
	attrs := map[string]any{
		"gen_ai.request.model": "gemini-pro",
		"ap.model": "gpt-4o",
		"gen_ai.usage.input_tokens": int64(100),
		"ap.prompt_tokens": int64(200),
		"gen_ai.usage.output_tokens": int64(50),
		"ap.completion_tokens": int64(80),
		"ap.span_type": "llm",
		"ap.cost_usd": float64(0.01),
		"ap.finish_reason": "length",
	}

	spanType, model, prompt, completion, total, cost, finishReason, _, _, _, _ :=
		MapSemanticConventions(attrs)

	if spanType != domain.SpanTypeLLM {
		t.Errorf("span_type = %s, want llm", spanType)
	}
	if model != "gpt-4o" {
		t.Errorf("model = %s, want gpt-4o (ap.* overrides GenAI)", model)
	}
	if prompt != 200 {
		t.Errorf("prompt_tokens = %d, want 200", prompt)
	}
	if completion != 80 {
		t.Errorf("completion_tokens = %d, want 80", completion)
	}
	if total != 280 {
		t.Errorf("total_tokens = %d, want 280", total)
	}
	if cost != 0.01 {
		t.Errorf("cost_usd = %.4f, want 0.01", cost)
	}
	if finishReason != "length" {
		t.Errorf("finish_reason = %s, want length", finishReason)
	}
}

func TestMapSemanticConventions_NegativeTokenGuard(t *testing.T) {
	attrs := map[string]any{
		"gen_ai.usage.input_tokens": int64(-1),
		"gen_ai.usage.output_tokens": int64(-5),
	}

	_, _, prompt, completion, total, _, _, _, _, _, _ :=
		MapSemanticConventions(attrs)

	if prompt != 0 {
		t.Errorf("negative prompt_tokens should be 0, got %d", prompt)
	}
	if completion != 0 {
		t.Errorf("negative completion_tokens should be 0, got %d", completion)
	}
	if total != 0 {
		t.Errorf("total_tokens should be 0 when both negative, got %d", total)
	}
}

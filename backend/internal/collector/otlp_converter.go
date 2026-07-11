// Package collector - OTLP 数据转换工具。
//
// 本文件提供 HTTP 和 gRPC handler 共用的 OTLP protobuf → domain.Span 转换逻辑。
// 设计原则：
//   - 纯函数，不依赖 HTTP/gRPC 上下文
//   - 一次编写，两处复用
package collector

import (
	"encoding/hex"
	"time"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/agentpulse/backend/internal/domain"
)

// ConvertOTLP 将 OTLP 请求数据转换为内部 Span 列表。
//
// 与 HTTPHandler.convertOTLP 逻辑完全一致，提取为独立函数供两处复用。
func ConvertOTLP(req *collectorpb.ExportTraceServiceRequest) []*domain.Span {
	var spans []*domain.Span

	for _, rs := range req.GetResourceSpans() {
		resourceAttrs := AttrsToMap(rs.GetResource().GetAttributes())
		serviceName := GetString(resourceAttrs, "service.name", "unknown")
		environment := GetString(resourceAttrs, "deployment.environment", "production")

		for _, ss := range rs.GetScopeSpans() {
			scopeName := ss.GetScope().GetName()
			_ = scopeName // 当前未使用，保留供未来扩展

			for _, otlpSpan := range ss.GetSpans() {
				span := SpanFromOTLP(otlpSpan, resourceAttrs, serviceName, environment)
				if span != nil {
					spans = append(spans, span)
				}
			}
		}
	}

	return spans
}

// SpanFromOTLP 将单个 OTLP Span 转换为 domain.Span。
func SpanFromOTLP(
	os *tracepb.Span,
	resourceAttrs map[string]any,
	serviceName, environment string,
) *domain.Span {
	spanAttrs := AttrsToMap(os.GetAttributes())

	span := &domain.Span{
		ID:           hex.EncodeToString(os.GetSpanId()),
		TraceID:      hex.EncodeToString(os.GetTraceId()),
		ParentSpanID: hex.EncodeToString(os.GetParentSpanId()),
		ServiceName:  serviceName,
		Environment:  environment,
		Name:         os.GetName(),
		StartTime:    time.Unix(0, int64(os.GetStartTimeUnixNano())),
		Status:       domain.SpanStatusOK,
		Attributes:   MergeAttrs(resourceAttrs, spanAttrs),
	}

	if os.GetEndTimeUnixNano() > 0 {
		span.EndTime = time.Unix(0, int64(os.GetEndTimeUnixNano()))
		span.LatencyMs = uint32(span.EndTime.Sub(span.StartTime).Milliseconds())
	}

	// 错误状态
	if os.Status != nil && os.Status.GetCode() == tracepb.Status_STATUS_CODE_ERROR {
		span.Status = domain.SpanStatusError
		if msg := os.Status.GetMessage(); msg != "" {
			span.ErrorMessage = msg
		}
	}

	// 从属性中提取 AgentPulse 自定义字段
	if v, ok := spanAttrs["ap.session_id"].(string); ok {
		span.SessionID = v
	}
	if v, ok := spanAttrs["ap.user_id"].(string); ok {
		span.UserID = v
	}
	if v, ok := spanAttrs["ap.agent_name"].(string); ok {
		span.AgentName = v
	}

	// 映射 OpenTelemetry GenAI 语义约定 (1.30+)
	span.Type, span.Model, span.PromptTokens, span.CompletionTokens, span.TotalTokens, span.CostUSD, span.FinishReason, span.ToolName, span.ReasoningStep, span.InputPreview, span.OutputPreview =
		MapSemanticConventions(spanAttrs)

	return span
}

// MapSemanticConventions 根据 OTel 语义约定映射字段。
//
// 返回 (spanType, model, promptTokens, completionTokens, totalTokens, costUSD, finishReason, toolName, reasoningStep, inputPreview, outputPreview)
func MapSemanticConventions(attrs map[string]any) (
	domain.SpanType, string, uint32, uint32, uint32, float64, string, string, uint16, string, string,
) {
	spanType := domain.SpanTypeAgent
	var (
		model            string
		promptTokens     uint32
		completionTokens uint32
		totalTokens      uint32
		costUSD          float64
		finishReason     string
		toolName         string
		reasoningStep    uint16
		inputPreview     string
		outputPreview    string
	)

	// GenAI 语义约定
	if v, ok := attrs["gen_ai.system"].(string); ok {
		_ = v
		// 不强制设置 type,留给 SDK 通过 ap.span_type 设置
	}
	if v, ok := attrs["gen_ai.response.model"].(string); ok {
		model = v
	} else if v, ok := attrs["gen_ai.request.model"].(string); ok {
		model = v
	}

	// 负值兜底: OTel IntValue 是 int64,负数强转 uint32 会变成巨大数字。
	if v, ok := attrs["gen_ai.usage.input_tokens"].(int64); ok && v >= 0 {
		promptTokens = uint32(v)
	}
	if v, ok := attrs["gen_ai.usage.output_tokens"].(int64); ok && v >= 0 {
		completionTokens = uint32(v)
	}
	totalTokens = promptTokens + completionTokens

	if v, ok := attrs["gen_ai.response.finish_reasons"].([]any); ok && len(v) > 0 {
		if s, ok := v[0].(string); ok {
			finishReason = s
		}
	}

	// AgentPulse 自定义字段优先级高于 OTel 约定
	if v, ok := attrs["ap.span_type"].(string); ok {
		spanType = domain.SpanType(v)
	}
	if v, ok := attrs["ap.model"].(string); ok && v != "" {
		model = v
	}
	if v, ok := attrs["ap.prompt_tokens"].(int64); ok && v >= 0 {
		promptTokens = uint32(v)
	}
	if v, ok := attrs["ap.completion_tokens"].(int64); ok && v >= 0 {
		completionTokens = uint32(v)
	}
	// ap.total_tokens 优先；缺失时回退到 prompt + completion 之和
	if v, ok := attrs["ap.total_tokens"].(int64); ok && v >= 0 {
		totalTokens = uint32(v)
	} else {
		totalTokens = promptTokens + completionTokens
	}
	if v, ok := attrs["ap.cost_usd"].(float64); ok && v >= 0 {
		costUSD = v
	}
	if v, ok := attrs["ap.finish_reason"].(string); ok {
		finishReason = v
	}
	if v, ok := attrs["ap.tool_name"].(string); ok {
		toolName = v
	}
	if v, ok := attrs["ap.reasoning_step"].(int64); ok && v >= 0 {
		reasoningStep = uint16(v)
	}
	if v, ok := attrs["ap.input_preview"].(string); ok {
		inputPreview = v
	}
	if v, ok := attrs["ap.output_preview"].(string); ok {
		outputPreview = v
	}

	return spanType, model, promptTokens, completionTokens, totalTokens, costUSD, finishReason, toolName, reasoningStep, inputPreview, outputPreview
}

// AttrsToMap 将 OTLP KeyValue 数组转为 map。
func AttrsToMap(kvs []*commonpb.KeyValue) map[string]any {
	if len(kvs) == 0 {
		return nil
	}
	m := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		m[kv.GetKey()] = AttrValueToAny(kv.GetValue())
	}
	return m
}

// AttrValueToAny 将 OTLP AnyValue 转为 Go 原生类型。
func AttrValueToAny(v *commonpb.AnyValue) any {
	if v == nil {
		return nil
	}
	switch x := v.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return x.StringValue
	case *commonpb.AnyValue_BoolValue:
		return x.BoolValue
	case *commonpb.AnyValue_IntValue:
		return x.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return x.DoubleValue
	case *commonpb.AnyValue_ArrayValue:
		arr := x.ArrayValue.GetValues()
		result := make([]any, len(arr))
		for i, item := range arr {
			result[i] = AttrValueToAny(item)
		}
		return result
	case *commonpb.AnyValue_KvlistValue:
		result := make(map[string]any)
		for _, kv := range x.KvlistValue.GetValues() {
			result[kv.GetKey()] = AttrValueToAny(kv.GetValue())
		}
		return result
	default:
		return nil
	}
}

// GetString 安全获取字符串属性。
func GetString(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

// MergeAttrs 合并两个属性 map（后者覆盖前者）。
func MergeAttrs(a, b map[string]any) map[string]any {
	if a == nil && b == nil {
		return nil
	}
	result := make(map[string]any, len(a)+len(b))
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

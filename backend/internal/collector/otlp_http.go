// Package collector 接收 OTLP 数据并转换为内部 Span 写入存储。
//
// 当前实现：
//   - HTTPHandler: 接收 OTLP/HTTP protobuf，转 Span 后异步批量写入
//
// 设计原则：
//   - 解耦 OTLP 协议与业务层：协议层只负责解析，业务层只看到 domain.Span
//   - 异步批写：不影响 OTLP 接收延迟
//   - 错误隔离：单条 Span 失败不影响整体
//
// 安全加固(v0.1.x):
//   - API Key 鉴权(基于 cfg.Auth.APIKeys 白名单)
//   - Body size 上限(默认 10MB,防止 OOM)
//   - panic recover(防止单个请求 panic 杀死进程)
//   - 失败写入通过 OTLP PartialSuccess 反馈客户端
package collector

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"runtime/debug"
	"time"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
)

// HTTPHandler 接收 OTLP/HTTP 请求。
//
// 路由：
//   POST /v1/traces - 接收 OTLP Trace 数据
//
// 协议：application/x-protobuf（OpenTelemetry 标准）
type HTTPHandler struct {
	services *service.Container
	cfg      *config.Config
	logger   logger.Logger
}

// NewHTTPHandler 创建 HTTP 处理器。
func NewHTTPHandler(cfg *config.Config, services *service.Container, log logger.Logger) http.Handler {
	if cfg == nil {
		panic("collector: config is required")
	}
	if services == nil {
		panic("collector: services container is required")
	}
	return &HTTPHandler{
		services: services,
		cfg:      cfg,
		logger:   log.WithFields(map[string]any{"component": "otlp_collector"}),
	}
}

// ServeHTTP 实现 http.Handler。
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// panic 恢复: 单个请求 panic 不能杀死整个进程。
	defer func() {
		if rec := recover(); rec != nil {
			h.logger.Errorf("otlp panic recovered: %v\n%s", rec, debug.Stack())
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}()

	// 仅接受 POST
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 仅接受 /v1/traces
	if r.URL.Path != "/v1/traces" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// 鉴权: 基于 X-AgentPulse-Key + cfg 白名单(常量时间比对)。
	requireKey := h.cfg.Auth.OTLPRequireKey == nil || *h.cfg.Auth.OTLPRequireKey
	if !config.ValidateAPIKey(h.cfg.APIKeysResolved(), requireKey, r.Header.Get("X-AgentPulse-Key")) {
		h.logger.WithFields(map[string]any{
			"client_ip": clientIP(r),
			"path":      r.URL.Path,
		}).Warnf("otlp auth failed: invalid or missing X-AgentPulse-Key")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// 限制 body 大小: 防止单次请求把内存撑爆。
	maxBody := h.cfg.OTLP.MaxBodySize
	if maxBody <= 0 {
		maxBody = 10 << 20 // 10MB fallback
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Errorf("read body: %v", err)
		http.Error(w, "request body too large or unreadable", http.StatusRequestEntityTooLarge)
		return
	}
	defer r.Body.Close()

	// 解析 OTLP protobuf
	req := &collectorpb.ExportTraceServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		h.logger.Errorf("unmarshal proto: %v", err)
		http.Error(w, "invalid OTLP protobuf", http.StatusBadRequest)
		return
	}

	// 转换为内部 Span
	spans := h.convertOTLP(req)
	if len(spans) == 0 {
		// 返回成功但无数据
		h.writeSuccess(w, 0, "")
		return
	}

	// 异步写入(不阻塞 OTLP 响应),失败通过 PartialSuccess 反馈。
	// 使用独立的 background ctx: r.Context() 在 handler 返回后立即取消,
	// 会导致异步写入在 ctx canceled 中失败。
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		defer func() {
			if rec := recover(); rec != nil {
				h.logger.Errorf("otlp async panic: %v\n%s", rec, debug.Stack())
			}
		}()
		if err := h.services.IngestSpans(ctx, spans); err != nil {
			h.logger.Errorf("ingest %d spans: %v", len(spans), err)
		}
	}()

	h.writeSuccess(w, len(spans), "")
}

// clientIP 提取客户端 IP(优先 X-Forwarded-For,其次 RemoteAddr)。
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}

// writeSuccess 写 OTLP 成功响应;若 rejected > 0 则通过 PartialSuccess 反馈失败原因。
func (h *HTTPHandler) writeSuccess(w http.ResponseWriter, count int, errMsg string) {
	resp := &collectorpb.ExportTraceServiceResponse{}

	if errMsg != "" {
		resp.PartialSuccess = &collectorpb.ExportTracePartialSuccess{
			RejectedSpans: int64(count),
			ErrorMessage:  errMsg,
		}
	} else {
		resp.PartialSuccess = &collectorpb.ExportTracePartialSuccess{
			RejectedSpans: 0,
			ErrorMessage:  "",
		}
	}

	data, err := proto.Marshal(resp)
	if err != nil {
		http.Error(w, "marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// convertOTLP 将 OTLP 数据转为 domain.Span。
//
// OTLP 数据模型：
//   ResourceSpans -> ScopeSpans -> Spans
//   每个 Span 包含 attributes (key-value)、events、links
func (h *HTTPHandler) convertOTLP(req *collectorpb.ExportTraceServiceRequest) []*domain.Span {
	var spans []*domain.Span

	for _, rs := range req.GetResourceSpans() {
		// Resource 属性(service.name, deployment.environment 等)
		resourceAttrs := h.attrsToMap(rs.GetResource().GetAttributes())
		serviceName := getString(resourceAttrs, "service.name", "unknown")
		environment := getString(resourceAttrs, "deployment.environment", "production")

		for _, ss := range rs.GetScopeSpans() {
			scopeName := ss.GetScope().GetName()

			for _, otlpSpan := range ss.GetSpans() {
				span := h.spanFromOTLP(otlpSpan, resourceAttrs, serviceName, environment, scopeName)
				if span != nil {
					spans = append(spans, span)
				}
			}
		}
	}

	return spans
}

func (h *HTTPHandler) spanFromOTLP(
	os *tracepb.Span,
	resourceAttrs map[string]any,
	serviceName, environment, scopeName string,
) *domain.Span {
	spanAttrs := h.attrsToMap(os.GetAttributes())

	span := &domain.Span{
		ID:           hex.EncodeToString(os.GetSpanId()),
		TraceID:      hex.EncodeToString(os.GetTraceId()),
		ParentSpanID: hex.EncodeToString(os.GetParentSpanId()),
		ServiceName:  serviceName,
		Environment:  environment,
		Name:         os.GetName(),
		StartTime:    time.Unix(0, int64(os.GetStartTimeUnixNano())),
		Status:       domain.SpanStatusOK,
		Attributes:   mergeAttrs(resourceAttrs, spanAttrs),
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
	// 这些字段由 SDK 设置,OTLP 通过 attributes 透传
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
	// 参考:https://opentelemetry.io/docs/specs/semconv/gen-ai/
	span.Type, span.Model, span.PromptTokens, span.CompletionTokens, span.TotalTokens, span.CostUSD, span.FinishReason, span.ToolName, span.ReasoningStep, span.InputPreview, span.OutputPreview =
		h.mapSemanticConventions(spanAttrs)

	return span
}

// mapSemanticConventions 根据 OTel 语义约定映射字段。
//
// 返回 (spanType, model, promptTokens, completionTokens, totalTokens, costUSD, finishReason, toolName, reasoningStep, inputPreview, outputPreview)
func (h *HTTPHandler) mapSemanticConventions(attrs map[string]any) (
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
	if v, ok := attrs["ap.total_tokens"].(int64); ok && v >= 0 {
		totalTokens = uint32(v)
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

// attrsToMap 将 OTLP KeyValue 数组转为 map。
func (h *HTTPHandler) attrsToMap(kvs []*commonpb.KeyValue) map[string]any {
	if len(kvs) == 0 {
		return nil
	}
	m := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		m[kv.GetKey()] = h.attrValueToAny(kv.GetValue())
	}
	return m
}

func (h *HTTPHandler) attrValueToAny(v *commonpb.AnyValue) any {
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
			result[i] = h.attrValueToAny(item)
		}
		return result
	case *commonpb.AnyValue_KvlistValue:
		result := make(map[string]any)
		for _, kv := range x.KvlistValue.GetValues() {
			result[kv.GetKey()] = h.attrValueToAny(kv.GetValue())
		}
		return result
	default:
		return nil
	}
}

// getString 安全获取字符串属性。
func getString(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

// mergeAttrs 合并两个属性 map(后者覆盖前者)。
func mergeAttrs(a, b map[string]any) map[string]any {
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
// Package collector 接收 OTLP 数据并转换为内部 Span 写入存储。
//
// 当前实现：
//   - HTTPHandler: 接收 OTLP/HTTP protobuf，转 Span 后同步入队
//   - GRPCHandler: 接收 OTLP/gRPC，转 Span 后同步入队
//
// 转换逻辑提取到 otlp_converter.go，两处 handler 共用。
//
// 安全加固(v0.1.x):
//   - API Key 鉴权(基于 cfg.Auth.APIKeys 白名单)
//   - Body size 上限(默认 10MB,防止 OOM)
//   - panic recover(防止单个请求 panic 杀死进程)
//   - 入队失败通过 OTLP PartialSuccess 反馈客户端（HTTP 仍 200 + PartialSuccess）
package collector

import (
	"context"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
)

// ingestTimeout 单次 Export 同步入队超时。
const ingestTimeout = 30 * time.Second

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

	// 转换为内部 Span（使用共享转换器，与 gRPC handler 逻辑一致）
	spans := ConvertOTLP(req)
	if len(spans) == 0 {
		// 返回成功但无数据
		h.writeSuccess(w, 0, "")
		return
	}

	// 同步入队：失败时通过 PartialSuccess 反馈，客户端可重试。
	// 批量落库仍由 SpanService worker 异步完成；此处保证「被接受」可观测。
	ctx, cancel := context.WithTimeout(r.Context(), ingestTimeout)
	defer cancel()
	if err := h.services.IngestSpans(ctx, spans); err != nil {
		h.logger.Errorf("ingest %d spans: %v", len(spans), err)
		h.writeSuccess(w, len(spans), "ingest failed: "+err.Error())
		return
	}

	h.writeSuccess(w, len(spans), "")
}

// clientIP 提取客户端 IP。
//
// 优先使用 X-Forwarded-For（仅取第一个 IP），
// 该 header 由可信反向代理设置；若无则使用 RemoteAddr。
// 注意：直接暴露给客户端的场景下 X-Forwarded-For 不可信，
// 生产环境应部署反向代理并配置可信代理白名单。
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// 取第一个 IP（最接近客户端的代理）
		if i := strings.Index(xff, ","); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	return r.RemoteAddr
}

// writeSuccess 写 OTLP 成功响应；errMsg 非空时通过 PartialSuccess 反馈失败原因。
func (h *HTTPHandler) writeSuccess(w http.ResponseWriter, count int, errMsg string) {
	resp := &collectorpb.ExportTraceServiceResponse{}

	if errMsg != "" {
		resp.PartialSuccess = &collectorpb.ExportTracePartialSuccess{
			RejectedSpans: int64(count),
			ErrorMessage:  errMsg,
		}
	}
	// errMsg == "" 时不设置 PartialSuccess，避免无意义字段。

	data, err := proto.Marshal(resp)
	if err != nil {
		http.Error(w, "marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

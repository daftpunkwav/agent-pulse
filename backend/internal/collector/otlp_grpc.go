// Package collector - gRPC OTLP 接收器。
//
// 实现标准 OTLP/gRPC Trace 服务：
//   service TraceService {
//     rpc Export(ExportTraceServiceRequest) returns (ExportTraceServiceResponse);
//   }
//
// 与 HTTPHandler 共用 otlp_converter.go 中的转换逻辑。
// 端口由 cfg.OTLP.GRPCPort 配置（默认 4317）。
package collector

import (
	"context"
	"runtime/debug"
	"strings"
	"time"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
)

// GRPCHandler 接收 OTLP/gRPC 请求。
//
// 嵌入 UnimplementedTraceServiceServer 以保证向前兼容。
type GRPCHandler struct {
	collectorpb.UnimplementedTraceServiceServer
	services *service.Container
	cfg      *config.Config
	logger   logger.Logger
}

// NewGRPCHandler 创建 gRPC 处理器。
//
// 实现 collectorpb.TraceServiceServer 接口，可直接注册到 gRPC server。
func NewGRPCHandler(cfg *config.Config, services *service.Container, log logger.Logger) collectorpb.TraceServiceServer {
	if cfg == nil {
		panic("collector: config is required")
	}
	if services == nil {
		panic("collector: services container is required")
	}
	return &GRPCHandler{
		services: services,
		cfg:      cfg,
		logger:   log.WithFields(map[string]any{"component": "otlp_grpc"}),
	}
}

// Export 实现 OTLP TraceService gRPC 接口。
//
// 处理流程：
//  1. 鉴权（X-AgentPulse-Key / gRPC metadata）
//  2. 转换 OTLP protobuf → domain.Span
//  3. 异步批量写入
//  4. 返回 PartialSuccess 结果
func (h *GRPCHandler) Export(ctx context.Context, req *collectorpb.ExportTraceServiceRequest) (*collectorpb.ExportTraceServiceResponse, error) {
	// panic 恢复
	defer func() {
		if rec := recover(); rec != nil {
			h.logger.Errorf("otlp grpc panic: %v\n%s", rec, debug.Stack())
		}
	}()

	// 鉴权: gRPC 通过 metadata 传递 API Key
	requireKey := h.cfg.Auth.OTLPRequireKey == nil || *h.cfg.Auth.OTLPRequireKey
	if requireKey {
		md, ok := metadata.FromIncomingContext(ctx)
		var apiKey string
		if ok {
			values := md.Get("x-agentpulse-key")
			if len(values) > 0 {
				apiKey = strings.TrimSpace(values[0])
			}
		}
		if !config.ValidateAPIKey(h.cfg.APIKeysResolved(), true, apiKey) {
			h.logger.Warnf("otlp grpc auth failed: invalid or missing X-AgentPulse-Key")
			return nil, status.Error(codes.Unauthenticated, "invalid or missing X-AgentPulse-Key")
		}
	}

	// 转换为内部 Span（与 HTTP handler 逻辑一致）
	spans := ConvertOTLP(req)
	if len(spans) == 0 {
		return &collectorpb.ExportTraceServiceResponse{}, nil
	}

	// 异步写入，不阻塞 gRPC 响应
	go func() {
		ingestCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		defer func() {
			if rec := recover(); rec != nil {
				h.logger.Errorf("otlp grpc async panic: %v\n%s", rec, debug.Stack())
			}
		}()
		if err := h.services.IngestSpans(ingestCtx, spans); err != nil {
			h.logger.Errorf("ingest %d spans (gRPC): %v", len(spans), err)
		}
	}()

	return &collectorpb.ExportTraceServiceResponse{}, nil
}

// Ensure GRPCHandler implements collectorpb.TraceServiceServer at compile time.
var _ collectorpb.TraceServiceServer = (*GRPCHandler)(nil)

// Package api - Trace Handler。
package api

import (
	"net/http"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// TraceHandler Trace 相关接口。
type TraceHandler struct {
	services *service.Container
	logger   logger.Logger
}

// NewTraceHandler 创建处理器。
func NewTraceHandler(services *service.Container, log logger.Logger) *TraceHandler {
	return &TraceHandler{
		services: services,
		logger:   log.WithFields(map[string]any{"component": "trace_handler"}),
	}
}

// GetTraceTree 查询完整调用树。
//
// GET /api/v1/traces/:trace_id
func (h *TraceHandler) GetTraceTree(c *gin.Context) {
	traceID := c.Param("trace_id")
	if traceID == "" {
		BadRequest(c, "trace_id is required")
		return
	}

	tree, err := h.services.SpanService.GetTraceTree(c.Request.Context(), traceID)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}
	if tree == nil {
		NotFound(c, "trace not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trace": tree,
	})
}

// ListBySession 查询会话下所有 Span。
func (h *TraceHandler) ListBySession(c *gin.Context) {
	sessionID := c.Param("session_id")
	opts := parseListOptions(c)

	spans, err := h.services.SpanService.ListBySession(c.Request.Context(), sessionID, opts)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"spans": spans,
		"count": len(spans),
	})
}

// ListByUser 查询用户下所有 Span。
func (h *TraceHandler) ListByUser(c *gin.Context) {
	userID := c.Param("user_id")
	opts := parseListOptions(c)

	spans, err := h.services.SpanRepo.ListByUser(c.Request.Context(), userID, opts)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"spans": spans,
		"count": len(spans),
	})
}

// ListByAgent 查询 Agent 所有 Span。
func (h *TraceHandler) ListByAgent(c *gin.Context) {
	agentName := c.Param("agent_name")
	opts := parseListOptions(c)

	spans, err := h.services.SpanRepo.ListByAgent(c.Request.Context(), agentName, opts)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"spans": spans,
		"count": len(spans),
	})
}

// ---------------------------------------------------------------------------
// 公共辅助
// ---------------------------------------------------------------------------

// parseListOptions 解析 query 参数到 ListOptions。
func parseListOptions(c *gin.Context) domain.ListOptions {
	opts := domain.ListOptions{
		Limit:     100,
		OrderBy:   "timestamp",
		OrderDesc: false,
	}

	if v := c.Query("limit"); v != "" {
		opts.Limit = parseIntDefault(v, 100)
	}
	if v := c.Query("offset"); v != "" {
		opts.Offset = parseIntDefault(v, 0)
	}
	if v := c.Query("status"); v != "" {
		opts.Status = domain.SpanStatus(v)
	}
	if v := c.Query("type"); v != "" {
		opts.Type = domain.SpanType(v)
	}
	if v := c.Query("from"); v != "" {
		if t, ok := parseTime(v); ok {
			opts.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, ok := parseTime(v); ok {
			opts.To = &t
		}
	}
	if v := c.Query("order_desc"); v == "true" {
		opts.OrderDesc = true
	}

	return opts
}


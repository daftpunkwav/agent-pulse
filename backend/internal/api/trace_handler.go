// Package api - Trace Handler.
package api

import (
	"net/http"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// TraceHandler serves trace-related endpoints.
type TraceHandler struct {
	services *service.Container
	logger   logger.Logger
}

// NewTraceHandler creates the handler.
func NewTraceHandler(services *service.Container, log logger.Logger) *TraceHandler {
	return &TraceHandler{
		services: services,
		logger:   log.WithFields(map[string]any{"component": "trace_handler"}),
	}
}

// GetTraceTree returns the full call tree of a trace.
//
// GET /api/v1/traces/:trace_id
func (h *TraceHandler) GetTraceTree(c *gin.Context) {
	traceID := c.Param("trace_id")
	if traceID == "" {
		BadRequest(c, "trace_id is required")
		return
	}
	// trace_id 是 32 位十六进制字符串（来自 OTLP），白名单约束防止下游问题。
	if !isValidHexTraceID(traceID) {
		BadRequest(c, "trace_id must be a 32-char hex string")
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

// ListBySession returns all spans in a session.
func (h *TraceHandler) ListBySession(c *gin.Context) {
	sessionID := c.Param("session_id")
	opts, ok := parseListOptions(c)
	if !ok {
		return
	}

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

// ListByUser returns all spans for a user.
func (h *TraceHandler) ListByUser(c *gin.Context) {
	userID := c.Param("user_id")
	opts, ok := parseListOptions(c)
	if !ok {
		return
	}

	spans, err := h.services.SpanService.ListByUser(c.Request.Context(), userID, opts)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"spans": spans,
		"count": len(spans),
	})
}

// ListByAgent returns all spans for an agent.
func (h *TraceHandler) ListByAgent(c *gin.Context) {
	agentName := c.Param("agent_name")
	opts, ok := parseListOptions(c)
	if !ok {
		return
	}

	spans, err := h.services.SpanService.ListByAgent(c.Request.Context(), agentName, opts)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"spans": spans,
		"count": len(spans),
	})
}

// parseListOptions parses query params into ListOptions, validating against
// the domain enum allow-lists. Returns ok=false if any param is invalid; the
// function has already written a 400 response to c in that case.
func parseListOptions(c *gin.Context) (domain.ListOptions, bool) {
	opts := domain.ListOptions{
		Limit:     100,
		OrderBy:   "timestamp",
		OrderDesc: false,
	}

	if v := c.Query("limit"); v != "" {
		n, ok := parseInt(v)
		if !ok || n <= 0 {
			BadRequest(c, "invalid limit")
			return opts, false
		}
		if n > 1000 {
			n = 1000
		}
		opts.Limit = n
	}
	if v := c.Query("offset"); v != "" {
		n, ok := parseInt(v)
		if !ok || n < 0 {
			BadRequest(c, "invalid offset")
			return opts, false
		}
		opts.Offset = n
	}
	if v := c.Query("status"); v != "" {
		s := domain.SpanStatus(v)
		if !domain.IsValidSpanStatus(s) {
			BadRequest(c, "invalid status: must be one of ok, error, timeout")
			return opts, false
		}
		opts.Status = s
	}
	if v := c.Query("type"); v != "" {
		t := domain.SpanType(v)
		if !domain.IsValidSpanType(t) {
			BadRequest(c, "invalid type: must be one of agent, llm, tool, reasoning, evaluation")
			return opts, false
		}
		opts.Type = t
	}
	if v := c.Query("from"); v != "" {
		t, ok := parseTime(v)
		if !ok {
			BadRequest(c, "invalid from (RFC3339 expected)")
			return opts, false
		}
		opts.From = &t
	}
	if v := c.Query("to"); v != "" {
		t, ok := parseTime(v)
		if !ok {
			BadRequest(c, "invalid to (RFC3339 expected)")
			return opts, false
		}
		opts.To = &t
	}
	if opts.From != nil && opts.To != nil && opts.From.After(*opts.To) {
		BadRequest(c, "from must be before to")
		return opts, false
	}
	if v := c.Query("order_by"); v != "" {
		if _, ok := domain.ValidOrderBy[v]; !ok {
			BadRequest(c, "invalid order_by: must be one of timestamp, cost, tokens, latency, start_time")
			return opts, false
		}
		opts.OrderBy = v
	}
	if v := c.Query("order_desc"); v == "true" {
		opts.OrderDesc = true
	}

	return opts, true
}

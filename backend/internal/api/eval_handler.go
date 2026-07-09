// Package api - Eval Handler.
package api

import (
	"net/http"

	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// EvalHandler handles evaluation endpoints.
type EvalHandler struct {
	services *service.Container
	logger   logger.Logger
}

// NewEvalHandler creates the handler.
func NewEvalHandler(services *service.Container, log logger.Logger) *EvalHandler {
	return &EvalHandler{
		services: services,
		logger:   log.WithFields(map[string]any{"component": "eval_handler"}),
	}
}

// AverageScores returns average dimension scores for an agent.
//
// GET /api/v1/eval/agents/:agent_name/scores
func (h *EvalHandler) AverageScores(c *gin.Context) {
	agentName := c.Param("agent_name")
	window, _ := parseWindow(c)

	scores, err := h.services.EvalService.AverageScores(c.Request.Context(), agentName, window)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent":  agentName,
		"window": window,
		"scores": scores,
	})
}

// GetBySpanID returns the evaluation for a specific span.
func (h *EvalHandler) GetBySpanID(c *gin.Context) {
	spanID := c.Param("span_id")

	eval, err := h.services.EvalRepo.GetBySpanID(c.Request.Context(), spanID)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}
	if eval == nil {
		NotFound(c, "evaluation not found for span")
		return
	}

	c.JSON(http.StatusOK, eval)
}

// ListByAgent lists all evaluations for an agent.
func (h *EvalHandler) ListByAgent(c *gin.Context) {
	agentName := c.Param("agent_name")
	opts, ok := parseListOptions(c)
	if !ok {
		return
	}
	evals, err := h.services.EvalService.ListByAgent(c.Request.Context(), agentName, opts)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"evaluations": evals,
		"count":       len(evals),
	})
}

// EvaluateNow synchronously triggers an evaluation for a span.
//
// POST /api/v1/eval/spans/:span_id
//
// Security:
//   - AuthMiddleware enforces X-AgentPulse-Key (mounted on /api/v1 group).
//   - Before sending to the LLM Judge, span.InputPreview/OutputPreview are
//     scrubbed via RedactPII to prevent PII (email, phone, JWT, api keys,
//     etc.) from being sent to the external Judge provider.
func (h *EvalHandler) EvaluateNow(c *gin.Context) {
	spanID := c.Param("span_id")

	span, err := h.services.SpanService.GetByID(c.Request.Context(), spanID)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}
	if span == nil {
		NotFound(c, "span not found")
		return
	}

	// PII redaction: this only affects the eval request, not the stored span.
	if span.InputPreview != "" {
		span.InputPreview = RedactPII(span.InputPreview)
	}
	if span.OutputPreview != "" {
		span.OutputPreview = RedactPII(span.OutputPreview)
	}

	eval, err := h.services.EvalService.EvaluateSync(c.Request.Context(), span)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	if err := h.services.EvalRepo.Insert(c.Request.Context(), eval); err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, eval)
}



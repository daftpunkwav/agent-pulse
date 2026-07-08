// Package api - Eval Handler。
package api

import (
	"net/http"

	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// EvalHandler 评估接口。
type EvalHandler struct {
	services *service.Container
	logger   logger.Logger
}

// NewEvalHandler 创建处理器。
func NewEvalHandler(services *service.Container, log logger.Logger) *EvalHandler {
	return &EvalHandler{
		services: services,
		logger:   log.WithFields(map[string]any{"component": "eval_handler"}),
	}
}

// AverageScores 查询维度平均分。
//
// GET /api/v1/eval/agents/:agent_name/scores
func (h *EvalHandler) AverageScores(c *gin.Context) {
	agentName := c.Param("agent_name")
	window, _ := parseWindow(c)

	scores, err := h.services.EvalService.AverageScores(c.Request.Context(), agentName, window)
	if err != nil {
		InternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent":  agentName,
		"window": window,
		"scores": scores,
	})
}

// GetBySpanID 根据 Span ID 查询评估。
func (h *EvalHandler) GetBySpanID(c *gin.Context) {
	spanID := c.Param("span_id")

	eval, err := h.services.EvalRepo.GetBySpanID(c.Request.Context(), spanID)
	if err != nil {
		InternalError(c, err)
		return
	}
	if eval == nil {
		NotFound(c, "evaluation not found for span")
		return
	}

	c.JSON(http.StatusOK, eval)
}

// ListByAgent 列出 Agent 所有评估。
func (h *EvalHandler) ListByAgent(c *gin.Context) {
	agentName := c.Param("agent_name")
	opts := parseListOptions(c)

	evals, err := h.services.EvalService.ListByAgent(c.Request.Context(), agentName, opts)
	if err != nil {
		InternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"evaluations": evals,
		"count":       len(evals),
	})
}

// EvaluateNow 立即触发评估（同步）。
//
// POST /api/v1/eval/spans/:span_id
func (h *EvalHandler) EvaluateNow(c *gin.Context) {
	spanID := c.Param("span_id")

	span, err := h.services.SpanService.GetByID(c.Request.Context(), spanID)
	if err != nil {
		InternalError(c, err)
		return
	}
	if span == nil {
		NotFound(c, "span not found")
		return
	}

	eval, err := h.services.EvalService.EvaluateSync(c.Request.Context(), span)
	if err != nil {
		InternalError(c, err)
		return
	}

	if err := h.services.EvalRepo.Insert(c.Request.Context(), eval); err != nil {
		InternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, eval)
}
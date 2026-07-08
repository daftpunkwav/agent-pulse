// Package api - Cost Handler。
package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// CostHandler 成本归因接口。
type CostHandler struct {
	services *service.Container
	logger   logger.Logger
}

// NewCostHandler 创建处理器。
func NewCostHandler(services *service.Container, log logger.Logger) *CostHandler {
	return &CostHandler{
		services: services,
		logger:   log.WithFields(map[string]any{"component": "cost_handler"}),
	}
}

// Breakdown 五维成本归因。
//
// GET /api/v1/cost/breakdown?from=&to=&dimensions=user,agent,tool&limit=10
func (h *CostHandler) Breakdown(c *gin.Context) {
	window, ok := parseWindow(c)
	if !ok {
		BadRequest(c, "from and to are required (RFC3339)")
		return
	}

	dimsParam := c.DefaultQuery("dimensions", "user,agent,tool,model")
	dims := parseDimensions(dimsParam)

	limit := parseInt(c.Query("limit"), 100)

	breakdowns, err := h.services.CostService.Breakdown(c.Request.Context(), window, dims, limit)
	if err != nil {
		InternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"window":     window,
		"breakdowns": breakdowns,
	})
}

// Timeline 成本时间序列。
//
// GET /api/v1/cost/timeline?from=&to=&granularity=hour|day
func (h *CostHandler) Timeline(c *gin.Context) {
	window, ok := parseWindow(c)
	if !ok {
		BadRequest(c, "from and to are required (RFC3339)")
		return
	}

	granularity := c.DefaultQuery("granularity", "hour")

	points, err := h.services.CostService.Timeline(c.Request.Context(), window, granularity)
	if err != nil {
		InternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"window":      window,
		"granularity": granularity,
		"points":      points,
	})
}

// Total 时间窗口内总成本。
//
// GET /api/v1/cost/total?from=&to=
func (h *CostHandler) Total(c *gin.Context) {
	window, ok := parseWindow(c)
	if !ok {
		BadRequest(c, "from and to are required (RFC3339)")
		return
	}

	totalCost, totalTokens, err := h.services.CostService.TotalCost(c.Request.Context(), window)
	if err != nil {
		InternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"window":       window,
		"total_usd":    totalCost,
		"total_tokens": totalTokens,
	})
}

// ---------------------------------------------------------------------------
// 辅助
// ---------------------------------------------------------------------------

// parseWindow 解析时间窗口。
func parseWindow(c *gin.Context) (domain.TimeWindow, bool) {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	if fromStr == "" || toStr == "" {
		// 默认最近 24h
		to := time.Now()
		from := to.Add(-24 * time.Hour)
		return domain.TimeWindow{From: from, To: to}, true
	}

	from, ok1 := parseTime(fromStr)
	to, ok2 := parseTime(toStr)
	if !ok1 || !ok2 {
		return domain.TimeWindow{}, false
	}

	return domain.TimeWindow{From: from, To: to}, true
}

// parseDimensions 解析维度列表。
func parseDimensions(s string) []domain.CostDimension {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]domain.CostDimension, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		result = append(result, domain.CostDimension(p))
	}
	return result
}
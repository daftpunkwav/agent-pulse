// Package api - Cost Handler銆?
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

// CostHandler 鎴愭湰褰掑洜鎺ュ彛銆?
type CostHandler struct {
	services *service.Container
	logger   logger.Logger
}

// NewCostHandler 鍒涘缓澶勭悊鍣ㄣ€?
func NewCostHandler(services *service.Container, log logger.Logger) *CostHandler {
	return &CostHandler{
		services: services,
		logger:   log.WithFields(map[string]any{"component": "cost_handler"}),
	}
}

// Breakdown 浜旂淮鎴愭湰褰掑洜銆?
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

	limit := parseIntDefault(c.Query("limit"), 100)

	breakdowns, err := h.services.CostService.Breakdown(c.Request.Context(), window, dims, limit)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"window":     window,
		"breakdowns": breakdowns,
	})
}

// Timeline 鎴愭湰鏃堕棿搴忓垪銆?
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
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"window":      window,
		"granularity": granularity,
		"points":      points,
	})
}

// Total 鏃堕棿绐楀彛鍐呮€绘垚鏈€?
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
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"window":       window,
		"total_usd":    totalCost,
		"total_tokens": totalTokens,
	})
}

// ---------------------------------------------------------------------------
// 杈呭姪
// ---------------------------------------------------------------------------

// parseWindow 瑙ｆ瀽鏃堕棿绐楀彛銆?
func parseWindow(c *gin.Context) (domain.TimeWindow, bool) {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	if fromStr == "" || toStr == "" {
		// 榛樿鏈€杩?24h
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

// parseDimensions 瑙ｆ瀽缁村害鍒楄〃銆?
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


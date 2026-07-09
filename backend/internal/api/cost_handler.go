// Package api - Cost Handler.
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

// CostHandler serves cost attribution endpoints.
type CostHandler struct {
	services *service.Container
	logger   logger.Logger
}

// NewCostHandler creates the handler.
func NewCostHandler(services *service.Container, log logger.Logger) *CostHandler {
	return &CostHandler{
		services: services,
		logger:   log.WithFields(map[string]any{"component": "cost_handler"}),
	}
}

// maxWindow is the largest window a single cost query may span.
// Prevents accidental "fetch last 5 years" requests from blowing up the DB.
const maxWindow = 90 * 24 * time.Hour

// Breakdown returns cost attribution across the requested dimensions.
//
// GET /api/v1/cost/breakdown?from=&to=&dimensions=user,agent,tool&limit=10
func (h *CostHandler) Breakdown(c *gin.Context) {
	window, ok := parseWindow(c)
	if !ok {
		return
	}

	dims, ok := parseDimensions(c, c.DefaultQuery("dimensions", "user,agent,tool,model"))
	if !ok {
		return
	}

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

// Timeline returns cost time-series.
//
// GET /api/v1/cost/timeline?from=&to=&granularity=hour|day
func (h *CostHandler) Timeline(c *gin.Context) {
	window, ok := parseWindow(c)
	if !ok {
		return
	}

	granularity := c.DefaultQuery("granularity", "hour")
	if granularity != "hour" && granularity != "day" {
		BadRequest(c, "granularity must be 'hour' or 'day'")
		return
	}

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

// Total returns total cost in a window.
//
// GET /api/v1/cost/total?from=&to=
func (h *CostHandler) Total(c *gin.Context) {
	window, ok := parseWindow(c)
	if !ok {
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

// parseWindow parses from/to query params into a TimeWindow.
//
// from and to are now REQUIRED (no silent 24h default). Window length is
// also capped at maxWindow to prevent accidental huge scans.
func parseWindow(c *gin.Context) (domain.TimeWindow, bool) {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	if fromStr == "" || toStr == "" {
		BadRequest(c, "from and to are required (RFC3339)")
		return domain.TimeWindow{}, false
	}

	from, ok1 := parseTime(fromStr)
	to, ok2 := parseTime(toStr)
	if !ok1 || !ok2 {
		BadRequest(c, "from/to must be valid RFC3339 timestamps")
		return domain.TimeWindow{}, false
	}

	if from.After(to) {
		BadRequest(c, "from must be before to")
		return domain.TimeWindow{}, false
	}

	if to.Sub(from) > maxWindow {
		BadRequest(c, "window length exceeds maximum (90 days)")
		return domain.TimeWindow{}, false
	}

	return domain.TimeWindow{From: from, To: to}, true
}

// parseDimensions parses a comma-separated list of cost dimensions and
// validates each against the allow-list. On invalid value, writes a 400 to c
// (when c is non-nil) and returns ok=false.
func parseDimensions(c *gin.Context, s string) ([]domain.CostDimension, bool) {
	if s == "" {
		return nil, true
	}
	parts := strings.Split(s, ",")
	result := make([]domain.CostDimension, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		d := domain.CostDimension(p)
		if !domain.IsValidCostDimension(d) {
			if c != nil {
				BadRequest(c, "invalid dimension: must be one of user, session, agent, tool, reasoning_step, model")
			}
			return nil, false
		}
		result = append(result, d)
	}
	return result, true
}

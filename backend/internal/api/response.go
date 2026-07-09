// Package api - common response helpers.
package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// BadRequest 400 error response.
func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error":      "bad_request",
		"message":    msg,
		"request_id": c.GetString("request_id"),
	})
}

// NotFound 404 error response.
func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, gin.H{
		"error":      "not_found",
		"message":    msg,
		"request_id": c.GetString("request_id"),
	})
}

// Unauthorized 401 error response.
func Unauthorized(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error":      "unauthorized",
		"message":    msg,
		"request_id": c.GetString("request_id"),
	})
}

// ServiceUnavailable 503 error response (dependency unavailable).
func ServiceUnavailable(c *gin.Context, msg string) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":      "service_unavailable",
		"message":    msg,
		"request_id": c.GetString("request_id"),
	})
}

// InternalError 500 error response.
//
// Client sees generic message + request_id only. Detailed err must be logged
// by the caller (use InternalErrorLog). Prevents leaking DB schema / SQL /
// stack info to clients.
func InternalError(c *gin.Context, err error) {
	rid := c.GetString("request_id")
	if rid == "" {
		rid = "unknown"
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":      "internal_error",
		"message":    "an internal error occurred, please retry with the request_id for support",
		"request_id": rid,
	})
}

// InternalErrorLog writes the err to logger and returns generic 500 to client.
func InternalErrorLog(c *gin.Context, log logger.Logger, err error) {
	rid := c.GetString("request_id")
	log.WithFields(map[string]any{
		"request_id": rid,
		"path":       c.Request.URL.Path,
		"method":     c.Request.Method,
		"client_ip":  c.ClientIP(),
	}).Errorf("internal error: %v", err)
	InternalError(c, err)
}

// parseInt parses an int; returns (0, false) on parse failure.
// Use parseIntDefault for optional params that should fall back to a default.
func parseInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseIntDefault parses an int; returns def on parse failure or empty string.
func parseIntDefault(s string, def int) int {
	if v, ok := parseInt(s); ok {
		return v
	}
	return def
}

// parseTime 解析 RFC3339 / RFC3339Nano 时间字符串。
//
// 前端 toISOString() 带毫秒，需兼容 RFC3339Nano。
func parseTime(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// isValidHexTraceID 校验 OTLP trace_id 格式：32 位十六进制。
func isValidHexTraceID(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// isShortSpanID 校验 OTLP span_id 格式：16 位十六进制。
func isShortSpanID(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

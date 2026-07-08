// Package api - 通用响应辅助。
package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// BadRequest 400 错误响应。
func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error":      "bad_request",
		"message":    msg,
		"request_id": c.GetString("request_id"),
	})
}

// NotFound 404 错误响应。
func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, gin.H{
		"error":      "not_found",
		"message":    msg,
		"request_id": c.GetString("request_id"),
	})
}

// InternalError 500 错误响应。
func InternalError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":      "internal_error",
		"message":    err.Error(),
		"request_id": c.GetString("request_id"),
	})
}

// parseInt 解析 int，失败返回默认值。
func parseInt(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}

// parseTime 解析 RFC3339 时间字符串。
func parseTime(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}
// Package api - 中间件。
package api

import (
	"net/http"
	"time"

	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDMiddleware 请求 ID 中间件。
//
// 每个请求分配唯一 ID，方便日志追踪。
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}

// LoggingMiddleware 日志中间件。
//
// 记录每个请求的 method/path/status/latency。
func LoggingMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		fields := map[string]any{
			"method":      method,
			"path":        path,
			"status":      statusCode,
			"latency_ms":  latency.Milliseconds(),
			"client_ip":   c.ClientIP(),
			"request_id":  c.GetString("request_id"),
		}

		// 错误请求用 Warn，否则 Info
		if statusCode >= 500 {
			log.WithFields(fields).Errorf("request failed")
		} else if statusCode >= 400 {
			log.WithFields(fields).Warnf("client error")
		} else {
			log.WithFields(fields).Infof("request completed")
		}
	}
}

// RecoveryMiddleware panic 恢复中间件。
//
// 避免单个请求的 panic 导致整个进程崩溃。
func RecoveryMiddleware(log logger.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		log.Errorf("panic recovered: %v", recovered)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":      "internal_server_error",
			"message":    "an unexpected error occurred",
			"request_id": c.GetString("request_id"),
		})
	})
}

// CORSMiddleware 跨域中间件。
//
// 开发环境默认允许所有来源。生产环境应改为白名单。
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Request-ID, X-AgentPulse-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// AuthMiddleware 鉴权中间件（API Key 鉴权，简化版）。
//
// 生产环境应替换为 JWT/OAuth2 等更严格的方案。
func AuthMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-AgentPulse-Key")
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "missing X-AgentPulse-Key header",
			})
			return
		}

		// TODO: 验证 API Key 有效性
		// 简化：直接放行，标记已鉴权
		c.Set("api_key", apiKey)
		c.Next()
	}
}

// HealthHandler 健康检查处理器。
func HealthHandler(services *service.Container, log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		// 注：当前 Application 还没有直接暴露 HealthCheck，
		// 简化：返回静态 OK
		_ = ctx
		_ = services
		_ = log

		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"version":   "0.1.0",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}
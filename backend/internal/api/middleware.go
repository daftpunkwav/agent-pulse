// Package api - middleware.
package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDMiddleware assigns a unique ID to each request for log tracing.
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

// LoggingMiddleware logs method/path/status/latency per request.
func LoggingMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		fields := map[string]any{
			"method":     method,
			"path":       path,
			"status":     statusCode,
			"latency_ms": latency.Milliseconds(),
			"client_ip":  c.ClientIP(),
			"request_id": c.GetString("request_id"),
		}

		// 5xx -> Error, 4xx -> Warn, else -> Info.
		if statusCode >= 500 {
			log.WithFields(fields).Errorf("request failed")
		} else if statusCode >= 400 {
			log.WithFields(fields).Warnf("client error")
		} else {
			log.WithFields(fields).Infof("request completed")
		}
	}
}

// RecoveryMiddleware recovers from panics so a single bad request cannot
// kill the whole process.
func RecoveryMiddleware(log logger.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		log.Errorf("panic recovered: %v", recovered)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":      "internal_error",
			"message":    "an unexpected error occurred",
			"request_id": c.GetString("request_id"),
		})
	})
}

// CORSMiddleware handles CORS with an explicit allow-list.
//
//  - When AllowedOrigins contains "*" and credentials are NOT used, "*" is sent.
//  - When a specific origin is configured, the request Origin is echoed with
//    Vary: Origin (so caches don't conflate responses).
//  - Allow-Origin: * + Allow-Credentials: true is never set (W3C forbids).
func CORSMiddleware(cfg *config.Config) gin.HandlerFunc {
	allowed := splitAndTrim(cfg.Server.AllowedOrigins)
	allowAny := false
	for _, o := range allowed {
		if o == "*" {
			allowAny = true
			break
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowedOrigin := ""
		switch {
		case allowAny:
			allowedOrigin = "*"
		case origin != "" && contains(allowed, origin):
			allowedOrigin = origin
			c.Writer.Header().Set("Vary", "Origin")
		}

		if allowedOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		}
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

// splitAndTrim splits by comma and trims spaces.
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	out := make([]string, 0, 4)
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			seg := trimSpace(s[start:i])
			if seg != "" {
				out = append(out, seg)
			}
			start = i + 1
		}
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// AuthMiddleware validates X-AgentPulse-Key against cfg.APIKeysResolved().
//
// When cfg.Auth.Enabled is false the middleware passes through (dev mode)
// but logs a warn to surface misconfiguration.
//
// On success the SHA-256 of the key is stored in context as api_key_hash
// for audit/rate-limit (never the plaintext).
func AuthMiddleware(cfg *config.Config, log logger.Logger) gin.HandlerFunc {
	allowed := make([][]byte, 0)
	for _, k := range cfg.APIKeysResolved() {
		sum := sha256.Sum256([]byte(k))
		allowed = append(allowed, sum[:])
	}

	return func(c *gin.Context) {
		if !cfg.Auth.Enabled {
			log.Warnf("auth disabled: %s %s passed without API key validation", c.Request.Method, c.Request.URL.Path)
			c.Next()
			return
		}

		apiKey := c.GetHeader("X-AgentPulse-Key")
		if apiKey == "" {
			Unauthorized(c, "missing X-AgentPulse-Key header")
			return
		}

		// Constant-time compare to avoid timing side channels.
		sum := sha256.Sum256([]byte(apiKey))
		matched := false
		for _, a := range allowed {
			if subtle.ConstantTimeCompare(sum[:], a) == 1 {
				matched = true
				break
			}
		}
		if !matched {
			log.WithFields(map[string]any{
				"client_ip":  c.ClientIP(),
				"path":       c.Request.URL.Path,
				"request_id": c.GetString("request_id"),
			}).Warnf("auth failed: invalid X-AgentPulse-Key")
			Unauthorized(c, "invalid API key")
			return
		}

		c.Set("api_key_hash", hexEncode(sum[:]))
		c.Next()
	}
}

// ValidateAPIKey checks the API key against the allow-list.
//
// requireKey=true: enforce; false: always pass (dev).
// Delegates to config.ValidateAPIKey to avoid collector/api import cycle.
func ValidateAPIKey(cfg *config.Config, apiKey string) bool {
	require := cfg.Auth.OTLPRequireKey == nil || *cfg.Auth.OTLPRequireKey
	return config.ValidateAPIKey(cfg.APIKeysResolved(), require, apiKey)
}

// hexEncode is a lower-case hex encoder used to store key hash in context.
func hexEncode(b []byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = hex[c>>4]
		out[i*2+1] = hex[c&0x0f]
	}
	return string(out)
}

// HealthPinger is the interface a real health-checker must implement.
// It is injected via service.Container.HealthPinger.
type HealthPinger interface {
	HealthCheck() error
}

// HealthHandler distinguishes liveness and readiness.
//
//   /healthz: liveness (process alive, no dependency check).
//   /readyz:  readiness (calls services.HealthPinger.HealthCheck).
//
// This separation is the K8s probe best practice.
func HealthHandler(services *service.Container, log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if path == "/healthz" {
			c.JSON(http.StatusOK, gin.H{
				"status":    "ok",
				"version":   "0.1.0",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		// /readyz: real dependency check
		if services == nil || services.HealthPinger == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":    "not_ready",
				"message":   "health pinger not configured",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		if err := services.HealthPinger.HealthCheck(); err != nil {
			log.WithField("error", err.Error()).Warnf("readiness check failed")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":    "not_ready",
				"error":     err.Error(),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"version":   "0.1.0",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// Package api - 限流中间件。
//
// 基于 Token Bucket 算法实现 IP 级请求限流：
//   - 每个 IP 独立限流，避免单 IP 耗尽配额
//   - 突发流量通过 burst 参数弹性处理
//   - 限流响应 429 Too Many Requests
package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// RateLimitConfig 限流配置。
type RateLimitConfig struct {
	// Enabled 是否启用限流。
	Enabled bool
	// Rate 每秒允许的请求数。
	Rate float64
	// Burst 突发请求配额（允许短时间内的峰值）。
	Burst int
}

// DefaultRateLimitConfig 返回默认限流配置。
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled: true,
		Rate:    10,   // 10 req/s
		Burst:   20,   // 允许 20 个突发请求
	}
}

// tokenBucket 单 IP 令牌桶。
type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
	burst      int
	rate       float64
	mu         sync.Mutex
}

// newTokenBucket 创建新令牌桶。
func newTokenBucket(rate float64, burst int) *tokenBucket {
	return &tokenBucket{
		tokens:     float64(burst),
		lastRefill: time.Now(),
		burst:      burst,
		rate:       rate,
	}
}

// allow 检查是否允许请求，返回 true 表示通过。
func (b *tokenBucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(b.tokens+elapsed*b.rate, float64(b.burst))
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// RateLimitMiddleware 返回 gin 限流中间件。
//
// 限流维度：客户端 IP（优先 X-Forwarded-For，否则 RemoteAddr）。
// 限流响应：429 + JSON `{"error":"rate_limit_exceeded","retry_after":<秒>}`。
func RateLimitMiddleware(cfg *config.Config, log logger.Logger) gin.HandlerFunc {
	rlCfg := DefaultRateLimitConfig()
	if cfg != nil && cfg.RateLimit != nil {
		rlCfg = RateLimitConfig{
			Enabled: cfg.RateLimit.Enabled,
			Rate:    cfg.RateLimit.Rate,
			Burst:   cfg.RateLimit.Burst,
		}
	}
	if !rlCfg.Enabled {
		// 限流关闭，直接返回空中间件
		return func(c *gin.Context) { c.Next() }
	}

	buckets := &sync.Map{} // map[string]*tokenBucket
	log.Infof("rate limiter enabled: rate=%.1f/s burst=%d", rlCfg.Rate, rlCfg.Burst)

	return func(c *gin.Context) {
		ip := clientIP(c.Request)
		bkt, _ := buckets.LoadOrStore(ip, newTokenBucket(rlCfg.Rate, rlCfg.Burst))

		if !bkt.(*tokenBucket).allow() {
			c.Header("Retry-After", "1")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"retry_after": 1,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// min 返回两个 float64 中的较小值。
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// clientIP 从 gin 请求中提取客户端 IP。
func clientIP(r *http.Request) string {
	// 优先 X-Forwarded-For（最接近客户端的 IP）
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	// 其次 X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// 最后 RemoteAddr（去掉端口）
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i >= 0 {
		ip = ip[:i]
	}
	return ip
}

// parseRateLimitConfig 从限流头解析客户端可见的限流信息。
//
// 用于响应头 X-RateLimit-Limit / X-RateLimit-Remaining / X-RateLimit-Reset。
func parseRateLimitConfig(cfg *config.RateLimitConfig) (rate float64, burst int) {
	if cfg == nil {
		return 10, 20
	}
	return cfg.Rate, cfg.Burst
}

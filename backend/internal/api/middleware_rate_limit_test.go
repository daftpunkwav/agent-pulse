// Package api - 限流中间件测试。
package api

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentpulse/backend/internal/config"
	"github.com/gin-gonic/gin"
)

func TestRateLimitMiddleware_AllowsRequestsUnderLimit(t *testing.T) {
	cfg := &config.Config{
		RateLimit: &config.RateLimitConfig{Enabled: true, Rate: 10, Burst: 5},
	}
	r := gin.New()
	r.Use(RateLimitMiddleware(cfg, nilLogger{}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// 前 burst 个请求应全部通过
	for i := 0; i < 5; i++ {
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("request %d: expected 200, got %d", i, w.Code)
		}
		w = httptest.NewRecorder()
	}
}

func TestRateLimitMiddleware_RejectsOverBurst(t *testing.T) {
	cfg := &config.Config{
		RateLimit: &config.RateLimitConfig{Enabled: true, Rate: 10, Burst: 2},
	}
	r := gin.New()
	r.Use(RateLimitMiddleware(cfg, nilLogger{}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	// 快速发送 burst+1 个请求
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		r.ServeHTTP(w, req)
		if i < 2 && w.Code != 200 {
			t.Errorf("request %d should pass (within burst), got %d", i, w.Code)
		}
		if i >= 2 && w.Code != 429 {
			t.Errorf("request %d should be rate limited, got %d", i, w.Code)
		}
	}
}

func TestRateLimitMiddleware_DifferentIPsIndependent(t *testing.T) {
	cfg := &config.Config{
		RateLimit: &config.RateLimitConfig{Enabled: true, Rate: 10, Burst: 1},
	}
	r := gin.New()
	r.Use(RateLimitMiddleware(cfg, nilLogger{}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	// IP A 打满 burst
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("first request should pass, got %d", w.Code)
	}

	// IP B 应该仍然可以访问
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.2:5678"
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("different IP should not be affected, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_Disabled(t *testing.T) {
	cfg := &config.Config{
		RateLimit: &config.RateLimitConfig{Enabled: false, Rate: 1, Burst: 1},
	}
	r := gin.New()
	r.Use(RateLimitMiddleware(cfg, nilLogger{}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	// 限流关闭时应无限制
	for i := 0; i < 100; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("request %d: rate limiting disabled, expected 200, got %d", i, w.Code)
			break
		}
	}
}

func TestRateLimitMiddleware_NilConfig(t *testing.T) {
	r := gin.New()
	r.Use(RateLimitMiddleware(nil, nilLogger{}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("nil config should use defaults, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_ResponseHeaders(t *testing.T) {
	cfg := &config.Config{
		RateLimit: &config.RateLimitConfig{Enabled: true, Rate: 10, Burst: 1},
	}
	r := gin.New()
	r.Use(RateLimitMiddleware(cfg, nilLogger{}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	// 打满 burst 后请求应返回 Retry-After header
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req) // 第一个通过

	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req) // 第二个应被拒

	if w.Code != 429 {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	retryAfter := w.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected Retry-After header on 429 response")
	}
}

func TestRateLimitMiddleware_TokenRefill(t *testing.T) {
	cfg := &config.Config{
		RateLimit: &config.RateLimitConfig{Enabled: true, Rate: 100, Burst: 1},
	}
	r := gin.New()
	r.Use(RateLimitMiddleware(cfg, nilLogger{}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	// 第一个请求通过
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal("first request should pass")
	}

	// 立即重试应被拒
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 429 {
		t.Error("second immediate request should be rate limited")
	}

	// 等待 100ms 后（rate=100/s → 10ms 恢复 1 token）应通过
	time.Sleep(100 * time.Millisecond)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("request after refill should pass, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_XForwardedFor(t *testing.T) {
	cfg := &config.Config{
		RateLimit: &config.RateLimitConfig{Enabled: true, Rate: 10, Burst: 1},
	}
	r := gin.New()
	r.Use(RateLimitMiddleware(cfg, nilLogger{}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	// 使用 X-Forwarded-For
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("first request with XFF should pass, got %d", w.Code)
	}

	// 不同 RemoteAddr 但相同 XFF 应被限流
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.RemoteAddr = "10.0.0.2:5678"
	r.ServeHTTP(w, req)
	if w.Code != 429 {
		t.Errorf("same XFF should share rate limit, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_XRealIP(t *testing.T) {
	cfg := &config.Config{
		RateLimit: &config.RateLimitConfig{Enabled: true, Rate: 10, Burst: 1},
	}
	r := gin.New()
	r.Use(RateLimitMiddleware(cfg, nilLogger{}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "203.0.113.5")
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("first request with X-Real-IP should pass, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "203.0.113.5")
	req.RemoteAddr = "10.0.0.2:5678"
	r.ServeHTTP(w, req)
	if w.Code != 429 {
		t.Errorf("same X-Real-IP should share rate limit, got %d", w.Code)
	}
}

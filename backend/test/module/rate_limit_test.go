// 模块测试：限流中间件并发安全与拒绝逻辑
package module_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/agentpulse/backend/internal/api"
	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

func TestRateLimitRejectsOverBurst(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		RateLimit: &config.RateLimitConfig{Enabled: true, Rate: 1, Burst: 2},
	}
	r := gin.New()
	r.Use(api.RateLimitMiddleware(cfg, logger.NewNop()))
	r.GET("/t", func(c *gin.Context) { c.Status(200) })

	var got429 int
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.RemoteAddr = "10.9.9.1:1234"
		r.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			got429++
		}
	}
	if got429 < 1 {
		t.Fatal("expected at least one 429 over burst")
	}
}

func TestRateLimitConcurrentSameIPNoRace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		RateLimit: &config.RateLimitConfig{Enabled: true, Rate: 100, Burst: 50},
	}
	r := gin.New()
	r.Use(api.RateLimitMiddleware(cfg, logger.NewNop()))
	r.GET("/t", func(c *gin.Context) { c.Status(200) })

	var ok atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/t", nil)
			req.RemoteAddr = "10.9.9.2:9999"
			r.ServeHTTP(w, req)
			if w.Code == 200 {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	// 并发下不应 panic；至少部分请求成功
	if ok.Load() < 1 {
		t.Fatal("expected some successful requests under concurrent load")
	}
}

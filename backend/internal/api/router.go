// Package api 提供 HTTP API 层。
//
// 路由分组：
//   /api/v1/trace/*     - Trace 查询
//   /api/v1/cost/*      - 成本归因
//   /api/v1/eval/*      - 评估结果
//   /api/v1/cluster/*   - 失败聚类
//   /api/v1/harness/*   - Harness 管理
//   /api/v1/abtest/*    - A/B 测试
//   /healthz, /readyz   - 健康检查
package api

import (
	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// NewRouter 创建并配置 HTTP 路由。
func NewRouter(cfg *config.Config, services *service.Container, log logger.Logger) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()

	// 全局中间件
	r.Use(RecoveryMiddleware(log))
	r.Use(LoggingMiddleware(log))
	r.Use(CORSMiddleware())
	r.Use(RequestIDMiddleware())

	// 健康检查（无需鉴权）
	r.GET("/healthz", HealthHandler(services, log))
	r.GET("/readyz", HealthHandler(services, log))

	// API v1
	v1 := r.Group("/api/v1")
	{
		// Trace
		traceHandler := NewTraceHandler(services, log)
		v1.GET("/traces/:trace_id", traceHandler.GetTraceTree)
		v1.GET("/sessions/:session_id/spans", traceHandler.ListBySession)
		v1.GET("/users/:user_id/spans", traceHandler.ListByUser)
		v1.GET("/agents/:agent_name/spans", traceHandler.ListByAgent)

		// Cost
		costHandler := NewCostHandler(services, log)
		v1.GET("/cost/breakdown", costHandler.Breakdown)
		v1.GET("/cost/timeline", costHandler.Timeline)
		v1.GET("/cost/total", costHandler.Total)

		// Eval
		evalHandler := NewEvalHandler(services, log)
		v1.GET("/eval/agents/:agent_name/scores", evalHandler.AverageScores)
		v1.GET("/eval/spans/:span_id", evalHandler.GetBySpanID)
		v1.GET("/eval/agents/:agent_name/list", evalHandler.ListByAgent)
		v1.POST("/eval/spans/:span_id", evalHandler.EvaluateNow)

		// Cluster
		clusterHandler := NewClusterHandler(services, log)
		v1.GET("/clusters", clusterHandler.List)
		v1.GET("/clusters/:cluster_id", clusterHandler.Get)
		v1.POST("/clusters/run", clusterHandler.RunAnalysis)

		// Harness
		harnessHandler := NewHarnessHandler(services, log)
		v1.GET("/harness/:agent_name/versions", harnessHandler.ListVersions)
		v1.GET("/harness/:agent_name/versions/:version", harnessHandler.GetVersion)
		v1.POST("/harness/:agent_name/versions", harnessHandler.CreateVersion)
		v1.POST("/harness/:agent_name/versions/:version/promote", harnessHandler.PromoteVersion)
		v1.GET("/harness/:agent_name/diff/:v1/:v2", harnessHandler.DiffVersions)

		// AB Test
		abHandler := NewABTestHandler(services, log)
		v1.GET("/abtests", abHandler.List)
		v1.GET("/abtests/:test_id", abHandler.Get)
		v1.POST("/abtests", abHandler.Create)
	}

	return r
}
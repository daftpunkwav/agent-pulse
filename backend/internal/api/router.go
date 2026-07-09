// Package api - HTTP router.
//
// Route groups:
//   /api/v1/traces/*    - Trace queries
//   /api/v1/cost/*      - Cost attribution
//   /api/v1/eval/*      - Evaluations
//   /api/v1/clusters/*  - Failure clustering
//   /api/v1/harness/*   - Harness management
//   /api/v1/abtests/*   - A/B tests
//   /healthz, /readyz   - Health checks
package api

import (
	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// NewRouter creates and configures the HTTP router.
func NewRouter(cfg *config.Config, services *service.Container, log logger.Logger) *gin.Engine {
	// gin.SetMode already called in app.initHTTPServers; do not call again.

	r := gin.New()

	// Global middleware
	r.Use(RecoveryMiddleware(log))
	r.Use(LoggingMiddleware(log))
	r.Use(CORSMiddleware(cfg))
	r.Use(RequestIDMiddleware())

	// Health checks (no auth required)
	r.GET("/healthz", HealthHandler(services, log))
	r.GET("/readyz", HealthHandler(services, log))

	// API v1
	v1 := r.Group("/api/v1")
	v1.Use(AuthMiddleware(cfg, log))
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

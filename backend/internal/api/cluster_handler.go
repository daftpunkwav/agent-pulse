// Package api - Cluster Handler。
package api

import (
	"net/http"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// ClusterHandler 失败聚类接口。
type ClusterHandler struct {
	services *service.Container
	logger   logger.Logger
}

// NewClusterHandler 创建处理器。
func NewClusterHandler(services *service.Container, log logger.Logger) *ClusterHandler {
	return &ClusterHandler{
		services: services,
		logger:   log.WithFields(map[string]any{"component": "cluster_handler"}),
	}
}

// List 列出所有聚类。
func (h *ClusterHandler) List(c *gin.Context) {
	activeOnly := c.DefaultQuery("active_only", "true") == "true"

	clusters, err := h.services.ClusterService.GetLatestClusters(c.Request.Context(), activeOnly)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}
	if clusters == nil {
		clusters = []*domain.FailureCluster{}
	}

	c.JSON(http.StatusOK, gin.H{
		"clusters": clusters,
		"count":    len(clusters),
	})
}

// Get 查询单个聚类。
func (h *ClusterHandler) Get(c *gin.Context) {
	clusterID := c.Param("cluster_id")
	if clusterID == "" {
		BadRequest(c, "cluster_id is required")
		return
	}

	cluster, err := h.services.ClusterService.GetCluster(c.Request.Context(), clusterID)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}
	if cluster == nil {
		NotFound(c, "cluster not found")
		return
	}

	c.JSON(http.StatusOK, cluster)
}

// RunAnalysis 手动触发聚类。
func (h *ClusterHandler) RunAnalysis(c *gin.Context) {
	window, ok := parseWindow(c)
	if !ok {
		return
	}

	clusters, err := h.services.ClusterService.RunAnalysis(c.Request.Context(), window)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"window":   window,
		"clusters": clusters,
		"count":    len(clusters),
	})
}

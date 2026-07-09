// Package api - Harness & AB Test Handler。
package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/internal/service"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// HarnessHandler Harness 配置接口。
type HarnessHandler struct {
	services *service.Container
	logger   logger.Logger
}

// NewHarnessHandler 创建处理器。
func NewHarnessHandler(services *service.Container, log logger.Logger) *HarnessHandler {
	return &HarnessHandler{
		services: services,
		logger:   log.WithFields(map[string]any{"component": "harness_handler"}),
	}
}

// ListVersions 列出 Agent 所有版本。
func (h *HarnessHandler) ListVersions(c *gin.Context) {
	agentName := c.Param("agent_name")

	versions, err := h.services.MetadataRepo.ListHarnessVersions(c.Request.Context(), agentName)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent":    agentName,
		"versions": versions,
		"count":    len(versions),
	})
}

// GetVersion 查询指定版本。
func (h *HarnessHandler) GetVersion(c *gin.Context) {
	agentName := c.Param("agent_name")
	version := parseIntDefault(c.Param("version"), 0)
	if version <= 0 {
		BadRequest(c, "invalid version")
		return
	}

	hc, err := h.services.MetadataRepo.GetHarnessVersion(c.Request.Context(), agentName, version)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}
	if hc == nil {
		NotFound(c, "harness version not found")
		return
	}

	c.JSON(http.StatusOK, hc)
}

// CreateVersionRequest 创建版本请求。
type CreateVersionRequest struct {
	ConfigYAML string `json:"config_yaml" binding:"required"`
	Notes      string `json:"notes"`
	CreatedBy  string `json:"created_by"`
}

// CreateVersion 创建新版本。
func (h *HarnessHandler) CreateVersion(c *gin.Context) {
	agentName := c.Param("agent_name")

	var req CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	hash := sha256.Sum256([]byte(req.ConfigYAML))
	hc := &domain.HarnessConfig{
		AgentName:  agentName,
		ConfigYAML: req.ConfigYAML,
		ConfigHash: hex.EncodeToString(hash[:]),
		Notes:      req.Notes,
		CreatedBy:  req.CreatedBy,
		Status:     domain.HarnessArchived,
	}

	if err := h.services.MetadataRepo.CreateHarnessVersion(c.Request.Context(), hc); err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusCreated, hc)
}

// PromoteVersion 提升版本到 production。
func (h *HarnessHandler) PromoteVersion(c *gin.Context) {
	agentName := c.Param("agent_name")
	version := parseIntDefault(c.Param("version"), 0)
	if version <= 0 {
		BadRequest(c, "invalid version")
		return
	}

	if err := h.services.MetadataRepo.UpdateHarnessStatus(c.Request.Context(), agentName, version, domain.HarnessProduction); err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent":   agentName,
		"version": version,
		"status":  domain.HarnessProduction,
	})
}

// DiffVersions 对比两个版本。
func (h *HarnessHandler) DiffVersions(c *gin.Context) {
	agentName := c.Param("agent_name")
	v1 := parseIntDefault(c.Param("v1"), 0)
	v2 := parseIntDefault(c.Param("v2"), 0)
	if v1 <= 0 || v2 <= 0 {
		BadRequest(c, "invalid version numbers")
		return
	}

	hc1, err := h.services.MetadataRepo.GetHarnessVersion(c.Request.Context(), agentName, v1)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}
	hc2, err := h.services.MetadataRepo.GetHarnessVersion(c.Request.Context(), agentName, v2)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	if hc1 == nil || hc2 == nil {
		NotFound(c, "one or both versions not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"v1":       hc1,
		"v2":       hc2,
		"same":     hc1.ConfigHash == hc2.ConfigHash,
	})
}

// ============================================================================
// AB Test Handler
// ============================================================================

// ABTestHandler A/B 测试接口。
type ABTestHandler struct {
	services *service.Container
	logger   logger.Logger
}

// NewABTestHandler 创建处理器。
func NewABTestHandler(services *service.Container, log logger.Logger) *ABTestHandler {
	return &ABTestHandler{
		services: services,
		logger:   log.WithFields(map[string]any{"component": "abtest_handler"}),
	}
}

// List 列出所有 A/B 测试。
func (h *ABTestHandler) List(c *gin.Context) {
	opts, ok := parseListOptions(c)
	if !ok {
		return
	}

	tests, err := h.services.MetadataRepo.ListABTests(c.Request.Context(), opts)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tests": tests,
		"count": len(tests),
	})
}

// Get 查询单个 A/B 测试。
func (h *ABTestHandler) Get(c *gin.Context) {
	id := c.Param("test_id")

	test, err := h.services.MetadataRepo.GetABTest(c.Request.Context(), id)
	if err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}
	if test == nil {
		NotFound(c, "ab test not found")
		return
	}

	c.JSON(http.StatusOK, test)
}

// CreateABTestRequest 创建请求。
type CreateABTestRequest struct {
	Name             string `json:"name" binding:"required"`
	AgentName        string `json:"agent_name" binding:"required"`
	ControlVersion   int    `json:"control_version" binding:"required"`
	TreatmentVersion int    `json:"treatment_version" binding:"required"`
	TrafficPercent   int    `json:"traffic_percent" binding:"required,min=1,max=100"`
}

// Create 创建 A/B 测试。
func (h *ABTestHandler) Create(c *gin.Context) {
	var req CreateABTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	test := &domain.ABTest{
		Name:             req.Name,
		AgentName:        req.AgentName,
		ControlVersion:   req.ControlVersion,
		TreatmentVersion: req.TreatmentVersion,
		TrafficPercent:   req.TrafficPercent,
		Status:           domain.ABTestRunning,
		StartedAt:        time.Now(),
	}

	if err := h.services.MetadataRepo.CreateABTest(c.Request.Context(), test); err != nil {
		InternalErrorLog(c, h.logger, err)
		return
	}

	c.JSON(http.StatusCreated, test)
}

// _ 防止 unused import 警告（strconv 用于 parseInt）
var _ = strconv.Itoa




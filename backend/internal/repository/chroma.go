// Package repository - Chroma 向量库客户端。
//
// Chroma 用于失败 Trace 的嵌入与聚类。
// 可选依赖：连接失败不阻塞启动。
package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agentpulse/backend/internal/config"
	"github.com/agentpulse/backend/pkg/logger"
)

// ChromaClient Chroma 向量库客户端（简化版 HTTP API）。
type ChromaClient struct {
	baseURL string
	apiKey  string
	tenant  string
	database string
	logger  logger.Logger
	httpClient *http.Client
}

// NewChromaClient 创建 Chroma 客户端。
func NewChromaClient(cfg config.ChromaConfig, log logger.Logger) (*ChromaClient, error) {
	baseURL := cfg.BaseURL()
	if baseURL == "" {
		return nil, fmt.Errorf("chroma host not configured")
	}

	client := &ChromaClient{
		baseURL:    baseURL,
		apiKey:     cfg.APIKey,
		tenant:     cfg.Tenant,
		database:   cfg.Database,
		logger:     log.WithFields(map[string]any{"component": "chroma"}),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	// 验证连接（Chroma v1 API）
	if err := client.Ping(); err != nil {
		return nil, fmt.Errorf("ping chroma: %w", err)
	}

	log.Infof("connected to chroma at %s", baseURL)
	return client, nil
}

// Ping 健康检查。
func (c *ChromaClient) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/heartbeat", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("chroma heartbeat status %d", resp.StatusCode)
	}
	return nil
}

// Close 关闭（HTTP 客户端无需显式关闭）。
func (c *ChromaClient) Close() error {
	return nil
}

// BaseURL 返回服务地址（仓储层使用）。
func (c *ChromaClient) BaseURL() string {
	return c.baseURL
}

// HTTPClient 返回 HTTP 客户端。
func (c *ChromaClient) HTTPClient() *http.Client {
	return c.httpClient
}

// Tenant 返回租户。
func (c *ChromaClient) Tenant() string {
	return c.tenant
}

// Database 返回数据库。
func (c *ChromaClient) Database() string {
	return c.database
}

func (c *ChromaClient) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.tenant != "" {
		req.Header.Set("X-Chroma-Tenant", c.tenant)
	}
	if c.database != "" {
		req.Header.Set("X-Chroma-Database", c.database)
	}
	if c.apiKey != "" {
		req.Header.Set("X-Chroma-Token", c.apiKey)
	}

	return req, nil
}
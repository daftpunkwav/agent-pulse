// Package repository - Chroma Vector 仓储实现。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/pkg/logger"
)

// ChromaVectorRepo VectorRepository 的 Chroma 实现。
type ChromaVectorRepo struct {
	client *ChromaClient
	logger logger.Logger
}

// NewChromaVectorRepo 创建仓储实例。
func NewChromaVectorRepo(client *ChromaClient, log logger.Logger) *ChromaVectorRepo {
	return &ChromaVectorRepo{
		client: client,
		logger: log.WithFields(map[string]any{"component": "vector_repo"}),
	}
}

// chromaCollection Chroma Collection 结构。
type chromaCollection struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ensureCollection 确保 Collection 存在。
//
// Chroma v2 API 需要先创建 collection 才能 add/query。
func (r *ChromaVectorRepo) ensureCollection(ctx context.Context, name string) error {
	// 检查是否存在
	req, err := r.client.newRequest(ctx, http.MethodGet,
		fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s",
			r.client.tenant, r.client.database, name), nil)
	if err != nil {
		return err
	}

	resp, err := r.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("check collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// 不存在则创建
	createBody := map[string]any{
		"name": name,
		"metadata": map[string]any{
			"description": "AgentPulse collection",
		},
	}

	req, err = r.client.newRequest(ctx, http.MethodPost,
		fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections",
			r.client.tenant, r.client.database), createBody)
	if err != nil {
		return err
	}

	resp, err = r.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create collection failed %d: %s", resp.StatusCode, body)
	}

	return nil
}

// Upsert 插入或更新向量。
func (r *ChromaVectorRepo) Upsert(
	ctx context.Context,
	collection string,
	id string,
	embedding []float32,
	metadata map[string]any,
) error {
	if err := r.ensureCollection(ctx, collection); err != nil {
		return err
	}

	body := map[string]any{
		"ids": []string{id},
		"embeddings": [][]float32{embedding},
		"metadatas": []map[string]any{metadata},
	}

	req, err := r.client.newRequest(ctx, http.MethodPost,
		fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/add",
			r.client.tenant, r.client.database, collection), body)
	if err != nil {
		return err
	}

	resp, err := r.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upsert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upsert failed %d: %s", resp.StatusCode, respBody)
	}

	return nil
}

// Query 查询相似向量。
func (r *ChromaVectorRepo) Query(
	ctx context.Context,
	collection string,
	embedding []float32,
	topK int,
) ([]domain.VectorMatch, error) {
	if topK <= 0 {
		topK = 10
	}

	body := map[string]any{
		"query_embeddings": [][]float32{embedding},
		"n_results":        topK,
	}

	req, err := r.client.newRequest(ctx, http.MethodPost,
		fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/query",
			r.client.tenant, r.client.database, collection), body)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query failed %d: %s", resp.StatusCode, respBody)
	}

	var raw struct {
		IDs       [][]string                `json:"ids"`
		Distances [][]float32               `json:"distances"`
		Metadatas [][]map[string]any        `json:"metadatas"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	var matches []domain.VectorMatch
	if len(raw.IDs) > 0 {
		for i, id := range raw.IDs[0] {
			match := domain.VectorMatch{
				ID:    id,
				Score: 0,
			}
			if i < len(raw.Distances[0]) {
				// Chroma 返回 distance，转 similarity
				match.Score = 1.0 - raw.Distances[0][i]
			}
			if i < len(raw.Metadatas[0]) {
				match.Metadata = raw.Metadatas[0][i]
			}
			matches = append(matches, match)
		}
	}

	return matches, nil
}

// Delete 删除向量。
func (r *ChromaVectorRepo) Delete(ctx context.Context, collection string, id string) error {
	body := map[string]any{
		"ids": []string{id},
	}

	req, err := r.client.newRequest(ctx, http.MethodPost,
		fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/delete",
			r.client.tenant, r.client.database, collection), body)
	if err != nil {
		return err
	}

	resp, err := r.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed %d: %s", resp.StatusCode, body)
	}

	return nil
}
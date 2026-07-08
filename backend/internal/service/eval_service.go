// Package service - 在线评估服务（LLM-as-Judge）。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/sashabaranov/go-openai"
)

// EvalService 评估服务。
//
// 关键职责：
//   - 接收 Span 触发评估（同步/采样）
//   - 调用 LLM-as-Judge 打分
//   - 持久化评估结果
type EvalService struct {
	evalRepo  domain.EvaluationRepository
	spanRepo  domain.SpanRepository
	logger    logger.Logger

	// LLM 客户端
	judgeClient *openai.Client
	judgeModel  string

	// 采样配置
	sampleRate float32

	// 异步队列
	queue    chan *evalJob
	workers  int
	wg       sync.WaitGroup
	closed   chan struct{}
	closeOnce sync.Once
}

type evalJob struct {
	span *domain.Span
	ctx  context.Context
}

// EvalServiceConfig 评估服务配置。
type EvalServiceConfig struct {
	Model       string
	APIKey      string
	BaseURL     string
	SampleRate  float32
	Workers     int
	QueueSize   int
}

// NewEvalService 创建评估服务实例。
func NewEvalService(
	evalRepo domain.EvaluationRepository,
	spanRepo domain.SpanRepository,
	log logger.Logger,
) *EvalService {
	cfg := EvalServiceConfig{
		Model:      "gpt-4o-mini",
		APIKey:     "",
		BaseURL:    "",
		SampleRate: 1.0,
		Workers:    3,
		QueueSize:  1000,
	}

	s := &EvalService{
		evalRepo:   evalRepo,
		spanRepo:   spanRepo,
		logger:     log.WithFields(map[string]any{"component": "eval_service"}),
		judgeModel: cfg.Model,
		sampleRate: cfg.SampleRate,
		queue:      make(chan *evalJob, cfg.QueueSize),
		closed:     make(chan struct{}),
	}

	if cfg.APIKey != "" {
		s.judgeClient = openai.NewClient(cfg.APIKey)
	}

	// 启动 worker
	for i := 0; i < cfg.Workers; i++ {
		s.wg.Add(1)
		go s.evalWorker(i)
	}

	return s
}

// SetJudge 设置 LLM Judge（延迟注入）。
func (s *EvalService) SetJudge(client *openai.Client, model string) {
	s.judgeClient = client
	s.judgeModel = model
}

// SetSampleRate 设置采样率。
func (s *EvalService) SetSampleRate(rate float32) {
	if rate < 0 || rate > 1 {
		return
	}
	s.sampleRate = rate
}

// EvaluateSync 同步评估单个 Span（阻塞等待结果）。
func (s *EvalService) EvaluateSync(ctx context.Context, span *domain.Span) (*domain.Evaluation, error) {
	return s.evaluate(ctx, span)
}

// EvaluateAsync 异步评估（不阻塞）。
//
// 根据 sampleRate 决定是否触发。
func (s *EvalService) EvaluateAsync(ctx context.Context, span *domain.Span) {
	if s.sampleRate < 1.0 && rand.Float32() > s.sampleRate {
		return
	}

	job := &evalJob{span: span, ctx: ctx}
	select {
	case s.queue <- job:
	case <-s.closed:
		s.logger.Warnf("eval service closed, drop span %s", span.ID)
	default:
		s.logger.Warnf("eval queue full, drop span %s", span.ID)
	}
}

// ListByAgent 查询 Agent 评估历史。
func (s *EvalService) ListByAgent(ctx context.Context, agentName string, opts domain.ListOptions) ([]*domain.Evaluation, error) {
	return s.evalRepo.ListByAgent(ctx, agentName, opts)
}

// AverageScores 查询维度平均分。
func (s *EvalService) AverageScores(
	ctx context.Context,
	agentName string,
	window domain.TimeWindow,
) (map[domain.EvaluationDimension]float32, error) {
	return s.evalRepo.AverageScores(ctx, agentName, window)
}

// ---------------------------------------------------------------------------
// 内部：评估执行
// ---------------------------------------------------------------------------

func (s *EvalService) evalWorker(id int) {
	defer s.wg.Done()

	for {
		select {
		case job := <-s.queue:
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			eval, err := s.evaluate(ctx, job.span)
			cancel()
			if err != nil {
				s.logger.Errorf("worker %d: evaluate span %s: %v", id, job.span.ID, err)
				continue
			}
			if err := s.evalRepo.Insert(ctx, eval); err != nil {
				s.logger.Errorf("worker %d: persist eval for span %s: %v", id, job.span.ID, err)
			}
		case <-s.closed:
			return
		}
	}
}

// evaluate 执行单次评估。
func (s *EvalService) evaluate(ctx context.Context, span *domain.Span) (*domain.Evaluation, error) {
	if s.judgeClient == nil {
		return nil, fmt.Errorf("judge client not configured")
	}

	input := &domain.JudgeInput{
		UserInput:   span.InputPreview,
		AgentOutput: span.OutputPreview,
		Metadata:    span.Attributes,
	}

	prompt := buildJudgePrompt(input)

	resp, err := s.judgeClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4oMini,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: judgeSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Temperature: 0.0,
	})
	if err != nil {
		return nil, fmt.Errorf("call judge: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty judge response")
	}

	// 解析 JSON 响应
	var scores struct {
		Accuracy       float32 `json:"accuracy"`
		Completeness   float32 `json:"completeness"`
		ToolSelection  float32 `json:"tool_selection"`
		ReasoningDepth float32 `json:"reasoning_depth"`
		Helpfulness    float32 `json:"helpfulness"`
		Rationale      string  `json:"rationale"`
	}

	content := resp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(content), &scores); err != nil {
		return nil, fmt.Errorf("parse judge response: %w", err)
	}

	eval := &domain.Evaluation{
		SpanID:         span.ID,
		TraceID:        span.TraceID,
		SessionID:      span.SessionID,
		UserID:         span.UserID,
		AgentName:      span.AgentName,
		Accuracy:       scores.Accuracy,
		Completeness:   scores.Completeness,
		ToolSelection:  scores.ToolSelection,
		ReasoningDepth: scores.ReasoningDepth,
		Helpfulness:    scores.Helpfulness,
		Rationale:      scores.Rationale,
		JudgeModel:     s.judgeModel,
		JudgePrompt:    prompt,
		Trigger:        domain.TriggerSync,
		SampleRate:     s.sampleRate,
		CreatedAt:      time.Now(),
	}

	// 计算总分（等权）
	eval.ComputeOverall(nil)

	return eval, nil
}

// Shutdown 优雅关闭。
func (s *EvalService) Shutdown(ctx context.Context) {
	s.closeOnce.Do(func() {
		close(s.closed)
		s.wg.Wait()
	})
}

// ---------------------------------------------------------------------------
// Judge Prompt 模板
// ---------------------------------------------------------------------------

const judgeSystemPrompt = `你是一位严格的 AI Agent 质量评估专家。
请根据用户的输入和 Agent 的输出，对以下五个维度进行 0-1 打分：
- accuracy: 事实是否正确
- completeness: 是否覆盖问题所有要点
- tool_selection: 工具选择是否合理（如果未提供工具调用信息，给 0.5）
- reasoning_depth: 推理是否充分
- helpfulness: 对用户是否有实际帮助

严格按 JSON 格式输出：{"accuracy": 0.0, "completeness": 0.0, "tool_selection": 0.0, "reasoning_depth": 0.0, "helpfulness": 0.0, "rationale": "..."}`

func buildJudgePrompt(input *domain.JudgeInput) string {
	prompt := fmt.Sprintf(`# 用户输入
%s

# Agent 输出
%s

请评估上述 Agent 回答的质量。`,
		input.UserInput,
		input.AgentOutput,
	)

	if len(input.ToolCalls) > 0 {
		prompt += "\n\n# 工具调用\n"
		for _, tc := range input.ToolCalls {
			prompt += fmt.Sprintf("- %s(args=%s, success=%v)\n", tc.Tool, tc.Args, tc.Success)
		}
	}

	return prompt
}
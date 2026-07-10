// Package domain 定义 AgentPulse 的领域模型与抽象接口。
//
// 本包是业务核心，不依赖任何外部实现（数据库、HTTP、第三方 SDK）。
// 所有 Repository、Service 都依赖本包定义的接口，实现层在 repository/ 和 service/ 中。
//
// 设计原则：
//   - 业务实体与持久化模型分离（domain.Span vs repository.clickhouseSpanRow）
//   - 接口由使用方定义（在 domain 中），实现在 repository/
//   - 字段含义清晰，配套 godoc 与业务示例
package domain

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// SpanType Span 类型枚举。
//
// 不同类型携带不同业务语义和必填字段：
//   - SpanTypeAgent: Agent 整体调用，包裹整个执行链
//   - SpanTypeLLM: 单次 LLM 调用，必填 model/tokens
//   - SpanTypeTool: 单次工具调用，必填 tool_name
//   - SpanTypeReasoning: 推理步骤，必填 step_index
//   - SpanTypeEvaluation: 评估结果，必填 dimensions 分数
type SpanType string

const (
	SpanTypeAgent      SpanType = "agent"
	SpanTypeLLM        SpanType = "llm"
	SpanTypeTool       SpanType = "tool"
	SpanTypeReasoning  SpanType = "reasoning"
	SpanTypeEvaluation SpanType = "evaluation"
)

// SpanStatus Span 状态枚举。
type SpanStatus string

const (
	SpanStatusOK      SpanStatus = "ok"
	SpanStatusError   SpanStatus = "error"
	SpanStatusTimeout SpanStatus = "timeout"
)

// Span 是 Agent 调用的最小可观测单元。
//
// 一个完整的 Agent 调用包含多个 Span，构成 Span Tree：
//
//	Session
//	  └── AgentCall Span (parent)
//	        ├── LLMCall Span (child)
//	        ├── ToolCall Span (child)
//	        ├── ReasoningStep Span (child)
//	        └── Evaluation Span (child, async)
//
// 字段说明：
//   - ID/TraceID/ParentSpanID 构成 Span Tree
//   - SessionID/UserID/AgentName 用于多维归因
//   - Type 决定必填字段（LLM 必填 model/tokens，Tool 必填 tool_name 等）
//   - LatencyMs 在 Span 结束时填充
//   - Attributes 存储类型特定的扩展字段（如 prompt 预览、tool 参数）
type Span struct {
	// 标识
	ID            string    `json:"id"`
	TraceID       string    `json:"trace_id"`
	ParentSpanID  string    `json:"parent_span_id,omitempty"`
	SessionID     string    `json:"session_id"`
	UserID        string    `json:"user_id"`
	AgentName     string    `json:"agent_name"`
	ServiceName   string    `json:"service_name"`
	Environment   string    `json:"environment"`

	// 类型与基础信息
	Type   SpanType   `json:"type"`
	Name   string     `json:"name"`
	Status SpanStatus `json:"status"`

	// 时间戳
	StartTime time.Time      `json:"start_time"`
	EndTime   time.Time      `json:"end_time"`
	LatencyMs uint32         `json:"latency_ms"`

	// LLM 字段（SpanType=llm 时必填）
	Model            string `json:"model,omitempty"`
	PromptTokens     uint32 `json:"prompt_tokens,omitempty"`
	CompletionTokens uint32 `json:"completion_tokens,omitempty"`
	TotalTokens      uint32 `json:"total_tokens,omitempty"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
	FinishReason     string  `json:"finish_reason,omitempty"`

	// Tool 字段（SpanType=tool 时必填）
	ToolName string `json:"tool_name,omitempty"`

	// Reasoning 字段（SpanType=reasoning 时必填）
	ReasoningStep uint16 `json:"reasoning_step,omitempty"`

	// 输入输出（用于调试，截断存储）
	InputPreview  string `json:"input_preview,omitempty"`
	OutputPreview string `json:"output_preview,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`

	// 扩展属性（JSON 字符串）
	Attributes map[string]any `json:"attributes,omitempty"`
}

// CalculateCost 根据当前价格表计算 LLM 调用成本。
//
// 用户可在外部修改 cost_usd 后调用此方法重算，或直接赋值。
func (s *Span) CalculateCost(pricing Pricing) {
	if s.Type != SpanTypeLLM {
		return
	}
	if pricing.PromptPrice <= 0 && pricing.CompletionPrice <= 0 {
		return
	}
	promptCost := float64(s.PromptTokens) / 1000.0 * pricing.PromptPrice
	completionCost := float64(s.CompletionTokens) / 1000.0 * pricing.CompletionPrice
	s.CostUSD = promptCost + completionCost
}

// Duration 返回 Span 耗时（EndTime - StartTime）。
func (s *Span) Duration() time.Duration {
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

// MarkComplete 标记 Span 完成，自动计算 LatencyMs。
func (s *Span) MarkComplete(status SpanStatus) {
	s.EndTime = time.Now()
	s.LatencyMs = uint32(s.Duration().Milliseconds())
	s.Status = status
}

// Validate 检查 Span 的必填字段是否完整。
//
// 不同 SpanType 有不同的必填字段要求：
//   - agent: 需 TraceID, SessionID
//   - llm:   需 TraceID, Model, PromptTokens >= 0
//   - tool:  需 TraceID, ToolName
//   - reasoning: 需 TraceID, ReasoningStep >= 0
//   - evaluation: 需 TraceID
func (s *Span) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("span id is required")
	}
	if s.TraceID == "" {
		return fmt.Errorf("span %s: trace_id is required", s.ID)
	}
	if s.SessionID == "" {
		return fmt.Errorf("span %s: session_id is required", s.ID)
	}
	if s.StartTime.IsZero() {
		return fmt.Errorf("span %s: start_time is required", s.ID)
	}

	switch s.Type {
	case SpanTypeLLM:
		if s.Model == "" {
			return fmt.Errorf("span %s: model is required for llm type", s.ID)
		}
	case SpanTypeTool:
		if s.ToolName == "" {
			return fmt.Errorf("span %s: tool_name is required for tool type", s.ID)
		}
	case SpanTypeReasoning:
		if s.ReasoningStep == 0 && s.ReasoningStep != ^uint16(0) {
			// reasoning_step 为 0 是合法的（第一步）
		}
	}

	return nil
}

// ============================================================================
// Evaluation 实体
// ============================================================================

// EvaluationDimension 评估维度枚举。
type EvaluationDimension string

const (
	DimensionAccuracy       EvaluationDimension = "accuracy"
	DimensionCompleteness   EvaluationDimension = "completeness"
	DimensionToolSelection  EvaluationDimension = "tool_selection"
	DimensionReasoningDepth EvaluationDimension = "reasoning_depth"
	DimensionHelpfulness    EvaluationDimension = "helpfulness"
)

// AllDimensions 返回所有标准维度。
func AllDimensions() []EvaluationDimension {
	return []EvaluationDimension{
		DimensionAccuracy,
		DimensionCompleteness,
		DimensionToolSelection,
		DimensionReasoningDepth,
		DimensionHelpfulness,
	}
}

// EvaluationTrigger 评估触发方式。
type EvaluationTrigger string

const (
	TriggerSync     EvaluationTrigger = "sync"     // 同步：每次 Trace 上报后立即
	TriggerSampled  EvaluationTrigger = "sampled"  // 采样：按概率触发
	TriggerOffline  EvaluationTrigger = "offline"  // 离线：批量回放评估
	TriggerFeedback EvaluationTrigger = "feedback" // 用户反馈触发
)

// Evaluation 是 LLM-as-Judge 对单次 Agent 调用的评估结果。
type Evaluation struct {
	ID        string `json:"id"`
	SpanID    string `json:"span_id"`
	TraceID   string `json:"trace_id"`
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	AgentName string `json:"agent_name"`

	// 五维评分（0-1）
	Accuracy       float32 `json:"accuracy"`
	Completeness   float32 `json:"completeness"`
	ToolSelection  float32 `json:"tool_selection"`
	ReasoningDepth float32 `json:"reasoning_depth"`
	Helpfulness    float32 `json:"helpfulness"`

	// 总分（加权平均）
	Overall float32 `json:"overall"`

	// 元数据
	Rationale     string            `json:"rationale"`
	JudgeModel    string            `json:"judge_model"`
	JudgePrompt   string            `json:"judge_prompt,omitempty"`
	Trigger       EvaluationTrigger `json:"trigger"`
	SampleRate    float32           `json:"sample_rate"`
	CreatedAt     time.Time         `json:"created_at"`
}

// Score 返回指定维度评分。
func (e *Evaluation) Score(dim EvaluationDimension) float32 {
	switch dim {
	case DimensionAccuracy:
		return e.Accuracy
	case DimensionCompleteness:
		return e.Completeness
	case DimensionToolSelection:
		return e.ToolSelection
	case DimensionReasoningDepth:
		return e.ReasoningDepth
	case DimensionHelpfulness:
		return e.Helpfulness
	default:
		return 0
	}
}

// ComputeOverall 计算总分（加权平均）。
func (e *Evaluation) ComputeOverall(weights map[EvaluationDimension]float32) float32 {
	if weights == nil {
		// 默认等权
		weights = map[EvaluationDimension]float32{
			DimensionAccuracy:       0.2,
			DimensionCompleteness:   0.2,
			DimensionToolSelection:  0.2,
			DimensionReasoningDepth: 0.2,
			DimensionHelpfulness:    0.2,
		}
	}
	var sum, totalWeight float32
	for dim, weight := range weights {
		sum += e.Score(dim) * weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		return 0
	}
	e.Overall = sum / totalWeight
	return e.Overall
}

// ============================================================================
// Cost 实体
// ============================================================================

// CostDimension 成本归因维度枚举。
type CostDimension string

const (
	DimensionUser      CostDimension = "user"
	DimensionSession   CostDimension = "session"
	DimensionAgent     CostDimension = "agent"
	DimensionTool      CostDimension = "tool"
	DimensionReasoning CostDimension = "reasoning_step"
	DimensionModel     CostDimension = "model"
)

// AllCostDimensions returns all cost dimensions.
func AllCostDimensions() []CostDimension {
	return []CostDimension{
		DimensionUser,
		DimensionSession,
		DimensionAgent,
		DimensionTool,
		DimensionReasoning,
		DimensionModel,
	}
}

// CostBreakdown 成本归因结果。
type CostBreakdown struct {
	Dimension CostDimension      `json:"dimension"`
	Items     []CostBreakdownItem `json:"items"`
	TotalUSD  float64            `json:"total_usd"`
	TotalTokens uint64           `json:"total_tokens"`
	Window    TimeWindow         `json:"window"`
}

// CostBreakdownItem 单个维度的归因项。
type CostBreakdownItem struct {
	Key       string  `json:"key"`        // 如 user_id/agent_name/tool_name
	CostUSD   float64 `json:"cost_usd"`
	Tokens    uint64  `json:"tokens"`
	CallCount uint64  `json:"call_count"`
	Rank      int     `json:"rank"`
}

// TimeWindow 时间窗口。
type TimeWindow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Pricing 模型价格（USD per 1k tokens）。
type Pricing struct {
	Model           string    `json:"model"`
	PromptPrice     float64   `json:"prompt_price"`
	CompletionPrice float64   `json:"completion_price"`
	Currency        string    `json:"currency"`
	EffectiveAt     time.Time `json:"effective_at"`
	ExpiredAt       *time.Time `json:"expired_at,omitempty"`
}

// ============================================================================
// FailureCluster 实体
// ============================================================================

// FailureCluster 表示一类失败模式。
type FailureCluster struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	TraceCount     int       `json:"trace_count"`
	Percentage     float32   `json:"percentage"`
	CommonPattern  string    `json:"common_pattern"`
	Suggestion     string    `json:"suggestion"`
	ExampleTraces  []string  `json:"example_traces"` // trace IDs
	Metadata       map[string]any `json:"metadata,omitempty"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ============================================================================
// 仓储接口（Repository）
// ============================================================================
//
// 仓储接口在 domain 层定义，实现在 repository/ 层。
// 使用方（service/handler）只依赖接口，便于替换存储实现和编写测试。

// SpanRepository Span 仓储接口。
type SpanRepository interface {
	// 写入
	Insert(ctx context.Context, span *Span) error
	BatchInsert(ctx context.Context, spans []*Span) error

	// 查询
	GetByID(ctx context.Context, id string) (*Span, error)
	GetByTraceID(ctx context.Context, traceID string) ([]*Span, error)
	ListBySession(ctx context.Context, sessionID string, opts ListOptions) ([]*Span, error)
	ListByUser(ctx context.Context, userID string, opts ListOptions) ([]*Span, error)
	ListByAgent(ctx context.Context, agentName string, opts ListOptions) ([]*Span, error)
	ListAllInWindow(ctx context.Context, opts ListOptions) ([]*Span, error)

	// Trace 回放
	GetTraceTree(ctx context.Context, traceID string) (*TraceTree, error)
}

// TraceTree 完整调用链。
type TraceTree struct {
	TraceID  string    `json:"trace_id"`
	SessionID string   `json:"session_id"`
	UserID   string    `json:"user_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Root      *Span     `json:"root"`
	AllSpans  []*Span   `json:"all_spans"`
	Depth     int       `json:"depth"`
}

// ListOptions 通用列表查询选项。
type ListOptions struct {
	From       *time.Time
	To         *time.Time
	Status     SpanStatus
	Type       SpanType
	AgentName  string
	UserID     string
	SessionID  string
	Limit      int
	Offset     int
	OrderBy    string // timestamp | cost | tokens
	OrderDesc  bool
}

// EvaluationRepository Evaluation 仓储接口。
type EvaluationRepository interface {
	Insert(ctx context.Context, eval *Evaluation) error
	BatchInsert(ctx context.Context, evals []*Evaluation) error

	GetByID(ctx context.Context, id string) (*Evaluation, error)
	GetBySpanID(ctx context.Context, spanID string) (*Evaluation, error)
	ListBySession(ctx context.Context, sessionID string) ([]*Evaluation, error)
	ListByAgent(ctx context.Context, agentName string, opts ListOptions) ([]*Evaluation, error)

	// 聚合查询
	AverageScores(ctx context.Context, agentName string, window TimeWindow) (map[EvaluationDimension]float32, error)
}

// PricingRepository 价格表仓储接口。
type PricingRepository interface {
	Get(ctx context.Context, model string, at time.Time) (*Pricing, error)
	ListActive(ctx context.Context) ([]*Pricing, error)
	Upsert(ctx context.Context, pricing *Pricing) error
}

// MetadataRepository 元数据仓储接口。
type MetadataRepository interface {
	// Harness 配置
	CreateHarnessVersion(ctx context.Context, hc *HarnessConfig) error
	GetHarnessVersion(ctx context.Context, agentName string, version int) (*HarnessConfig, error)
	ListHarnessVersions(ctx context.Context, agentName string) ([]*HarnessConfig, error)
	UpdateHarnessStatus(ctx context.Context, agentName string, version int, status HarnessStatus) error

	// A/B 测试
	CreateABTest(ctx context.Context, ab *ABTest) error
	GetABTest(ctx context.Context, id string) (*ABTest, error)
	ListABTests(ctx context.Context, opts ListOptions) ([]*ABTest, error)
	UpdateABTestResult(ctx context.Context, id string, result *ABTestResult) error

	// 失败聚类
	InsertFailureCluster(ctx context.Context, cluster *FailureCluster) error
	ListFailureClusters(ctx context.Context, activeOnly bool) ([]*FailureCluster, error)
	GetFailureClusterByID(ctx context.Context, id string) (*FailureCluster, error)
}

// VectorRepository 向量仓储接口（用于失败聚类）。
type VectorRepository interface {
	Upsert(ctx context.Context, collection string, id string, embedding []float32, metadata map[string]any) error
	Query(ctx context.Context, collection string, embedding []float32, topK int) ([]VectorMatch, error)
	Delete(ctx context.Context, collection string, id string) error
}

// VectorMatch 向量匹配结果。
type VectorMatch struct {
	ID       string         `json:"id"`
	Score    float32        `json:"score"`
	Metadata map[string]any `json:"metadata"`
}

// ============================================================================
// Harness / A/B Test 实体
// ============================================================================

// HarnessStatus Harness 配置状态。
type HarnessStatus string

const (
	HarnessProduction HarnessStatus = "production"
	HarnessCanary     HarnessStatus = "canary"
	HarnessArchived   HarnessStatus = "archived"
)

// HarnessConfig Agent Harness 配置的一个版本。
type HarnessConfig struct {
	ID             string        `json:"id"`
	AgentName      string        `json:"agent_name"`
	Version        int           `json:"version"`
	ConfigYAML     string        `json:"config_yaml"`
	ConfigHash     string        `json:"config_hash"`
	Status         HarnessStatus `json:"status"`
	TrafficPercent int           `json:"traffic_percent"`
	Notes          string        `json:"notes,omitempty"`
	CreatedBy      string        `json:"created_by,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	PromotedAt     *time.Time    `json:"promoted_at,omitempty"`
}

// ABTest A/B 测试。
type ABTest struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	AgentName        string       `json:"agent_name"`
	ControlVersion   int          `json:"control_version"`
	TreatmentVersion int          `json:"treatment_version"`
	TrafficPercent   int          `json:"traffic_percent"`
	Status           ABTestStatus `json:"status"`
	StartedAt        time.Time    `json:"started_at"`
	EndedAt          *time.Time   `json:"ended_at,omitempty"`
	Result           *ABTestResult `json:"result,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
}

// ABTestStatus A/B 测试状态。
type ABTestStatus string

const (
	ABTestRunning   ABTestStatus = "running"
	ABTestCompleted ABTestStatus = "completed"
	ABTestAborted   ABTestStatus = "aborted"
)

// ABTestResult A/B 测试结果。
type ABTestResult struct {
	Winner           string                  `json:"winner"` // "control" | "treatment" | "inconclusive"
	ControlMetrics   ABTestMetrics           `json:"control_metrics"`
	TreatmentMetrics ABTestMetrics           `json:"treatment_metrics"`
	PValue           float64                 `json:"p_value"`
	ConfidenceLevel  float32                 `json:"confidence_level"`
	Recommendation   string                  `json:"recommendation"`
	ComputedAt       time.Time               `json:"computed_at"`
}

// ABTestMetrics A/B 测试指标。
type ABTestMetrics struct {
	SampleSize        uint64                          `json:"sample_size"`
	SuccessRate       float32                         `json:"success_rate"`
	AvgCostUSD        float64                         `json:"avg_cost_usd"`
	AvgLatencyMs      float32                         `json:"avg_latency_ms"`
	AvgOverall        float32                         `json:"avg_overall"`
	DimensionAverages map[EvaluationDimension]float32 `json:"dimension_averages"`
}

// ============================================================================
// LLM Judge 接口
// ============================================================================

// JudgeInput Judge 输入数据。
type JudgeInput struct {
	UserInput  string                 `json:"user_input"`
	AgentOutput string                `json:"agent_output"`
	Trace      []*Span                `json:"trace,omitempty"`
	ToolCalls  []ToolCallSummary      `json:"tool_calls,omitempty"`
	Metadata   map[string]any         `json:"metadata,omitempty"`
}

// ToolCallSummary 工具调用摘要（评估时使用）。
type ToolCallSummary struct {
	Tool    string `json:"tool"`
	Args    string `json:"args"`
	Result  string `json:"result,omitempty"`
	LatencyMs uint32 `json:"latency_ms"`
	Success  bool   `json:"success"`
}

// JudgeOutput Judge 输出。
type JudgeOutput struct {
	Scores     map[EvaluationDimension]float32 `json:"scores"`
	Rationale  string                          `json:"rationale"`
	TokensUsed int                             `json:"tokens_used"`
}

// Judge LLM-as-Judge 抽象接口。
//
// 多种实现：
//   - OpenAI Judge（GPT-4o）
//   - Anthropic Judge（Claude）
//   - DeepSeek Judge
//   - Custom Judge（自研评估 Prompt）
//
// 通过注册机制在 service 层组合。
type Judge interface {
	// Name 返回 Judge 名称（用于日志和指标）。
	Name() string

	// Evaluate 对单次 Agent 调用打分。
	Evaluate(ctx context.Context, input *JudgeInput) (*JudgeOutput, error)

	// Close 释放资源。
	Close() error
}

// ============================================================================
// 聚类服务接口
// ============================================================================

// FailureClusterService 失败聚类服务接口。
type FailureClusterService interface {
	// RunAnalysis 对指定时间窗口内的失败 Trace 执行聚类分析。
	RunAnalysis(ctx context.Context, window TimeWindow) ([]*FailureCluster, error)

	// GetLatestClusters 获取聚类列表。
	GetLatestClusters(ctx context.Context, activeOnly bool) ([]*FailureCluster, error)

	// GetCluster 获取单个聚类详情。
	GetCluster(ctx context.Context, id string) (*FailureCluster, error)
}



// ============================================================================
// Sentinel Errors
// ============================================================================
//
// Use errors.Is to detect these in handlers and return appropriate HTTP
// status codes (ErrNotFound -> 404, ErrInvalidInput -> 400, etc.).
var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrServiceClosed = errors.New("service closed")
	ErrUnauthorized  = errors.New("unauthorized")
)

// ValidSpanTypes lists all legal SpanType values for input validation.
var ValidSpanTypes = map[SpanType]struct{}{
	SpanTypeAgent:      {},
	SpanTypeLLM:        {},
	SpanTypeTool:       {},
	SpanTypeReasoning:  {},
	SpanTypeEvaluation: {},
}

// ValidSpanStatuses lists all legal SpanStatus values.
var ValidSpanStatuses = map[SpanStatus]struct{}{
	SpanStatusOK:      {},
	SpanStatusError:   {},
	SpanStatusTimeout: {},
}

// IsValidSpanType reports whether t is a known SpanType.
func IsValidSpanType(t SpanType) bool {
	_, ok := ValidSpanTypes[t]
	return ok
}

// IsValidSpanStatus reports whether s is a known SpanStatus.
func IsValidSpanStatus(s SpanStatus) bool {
	_, ok := ValidSpanStatuses[s]
	return ok
}

// ValidOrderBy lists the columns allowed in SpanListOptions.OrderBy.
// The query builder refuses any other value (SQL injection guard).
var ValidOrderBy = map[string]struct{}{
	"timestamp":  {},
	"cost":       {},
	"tokens":     {},
	"latency":    {},
	"start_time": {},
}



// ValidCostDimensions is the allow-list for CostDimension input validation.
var ValidCostDimensions = map[CostDimension]struct{}{
	DimensionUser:      {},
	DimensionSession:   {},
	DimensionAgent:     {},
	DimensionTool:      {},
	DimensionReasoning: {},
	DimensionModel:     {},
}

// IsValidCostDimension reports whether d is a known CostDimension.
func IsValidCostDimension(d CostDimension) bool {
	_, ok := ValidCostDimensions[d]
	return ok
}

// ============================================================================
// ClickHouse Query Executor Interface
// ============================================================================
//
// CostService 等业务层依赖此接口执行 ClickHouse 查询，
// 避免直接 import clickhouse-go 驱动，实现仓储层与业务层解耦。
// 实现方: repository.ClickHouseClient（已实现 QueryRows / QueryRow / Conn）

// ClickHouseQueryExecutor 抽象 ClickHouse 查询执行能力。
//
// 设计原则：
//   - 接口由使用方 (CostService) 定义，符合"接口由使用方定义"原则
//   - 不引入 clickhouse-go driver 依赖（回调模式替代 driver.Rows）
//   - QueryRow 通过回调返回单行结果，保持与 QueryRows 一致的错误处理
type ClickHouseQueryExecutor interface {
	// QueryRows 查询多行，scanFn 处理每行数据。
	// rows.Err() 在迭代完成后检查。
	QueryRows(ctx context.Context, query string, scanFn func(rows Rows) error, args ...any) error

	// QueryRow 查询单行，scanFn 处理结果行。
	QueryRow(ctx context.Context, query string, scanFn func(row Row) error, args ...any) error

	// Conn 返回底层连接（用于 Exec 等原生操作）。
	Conn() Conn
}

// Rows 抽象数据库行迭代器，避免 domain 包依赖 clickhouse-go driver。
type Rows interface {
	// Next 推进到下一行，返回 false 表示无更多行或出错。
	Next() bool
	// Scan 将当前行扫描到目标变量。
	Scan(dest ...any) error
	// Err 返回迭代过程中的错误（需在 Next() 返回 false 后调用）。
	Err() error
	// Close 释放行资源。
	Close() error
}

// Row 抽象单行结果。
type Row interface {
	// Scan 将行扫描到目标变量。
	Scan(dest ...any) error
}

// Conn 抽象底层数据库连接。
type Conn interface {
	// Exec 执行无返回结果的 SQL。
	Exec(ctx context.Context, query string, args ...any) error
}



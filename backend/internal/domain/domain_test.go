// Package domain_test 验证领域模型的业务逻辑与校验。
package domain_test

import (
	"testing"
	"time"

	"github.com/agentpulse/backend/internal/domain"
)

// ===== Span 实体测试 =====

func TestSpanDuration(t *testing.T) {
	start := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 1, 12, 0, 5, 0, time.UTC)

	t.Run("completed span", func(t *testing.T) {
		span := &domain.Span{
			StartTime: start,
			EndTime:   end,
		}
		d := span.Duration()
		if d != 5*time.Second {
			t.Errorf("Duration = %v, want 5s", d)
		}
	})

	t.Run("in-progress span returns elapsed", func(t *testing.T) {
		span := &domain.Span{
			StartTime: time.Now().Add(-3 * time.Second),
			EndTime:   time.Time{},
		}
		d := span.Duration()
		if d < 2*time.Second || d > 4*time.Second {
			t.Errorf("Duration = %v, expected ~3s", d)
		}
	})
}

func TestSpanMarkComplete(t *testing.T) {
	before := time.Now().Add(-100 * time.Millisecond)
	span := &domain.Span{StartTime: before}

	span.MarkComplete(domain.SpanStatusOK)

	if span.Status != domain.SpanStatusOK {
		t.Errorf("Status = %q, want %q", span.Status, domain.SpanStatusOK)
	}
	if span.EndTime.IsZero() {
		t.Error("EndTime should be set after MarkComplete")
	}
	if span.LatencyMs == 0 {
		t.Error("LatencyMs should be > 0 after MarkComplete")
	}
	if span.EndTime.Before(before) {
		t.Error("EndTime should be after StartTime")
	}
}

func TestSpanCalculateCost(t *testing.T) {
	t.Run("non-LLM span skips cost calculation", func(t *testing.T) {
		span := &domain.Span{Type: domain.SpanTypeTool, CostUSD: 0}
		span.CalculateCost(domain.Pricing{PromptPrice: 0.01, CompletionPrice: 0.02})
		if span.CostUSD != 0 {
			t.Errorf("non-LLM span CostUSD should remain 0, got %f", span.CostUSD)
		}
	})

	t.Run("zero pricing skips calculation", func(t *testing.T) {
		span := &domain.Span{
			Type:            domain.SpanTypeLLM,
			PromptTokens:    1000,
			CompletionTokens: 500,
			CostUSD:         0,
		}
		span.CalculateCost(domain.Pricing{PromptPrice: 0, CompletionPrice: 0})
		if span.CostUSD != 0 {
			t.Errorf("CostUSD should remain 0 with zero pricing, got %f", span.CostUSD)
		}
	})

	t.Run("cost calculated correctly", func(t *testing.T) {
		span := &domain.Span{
			Type:            domain.SpanTypeLLM,
			PromptTokens:    1000,
			CompletionTokens: 500,
			CostUSD:         0,
		}
		// 1000/1000 * 0.01 + 500/1000 * 0.02 = 0.01 + 0.01 = 0.02
		span.CalculateCost(domain.Pricing{PromptPrice: 0.01, CompletionPrice: 0.02})
		if span.CostUSD != 0.02 {
			t.Errorf("CostUSD = %f, want 0.02", span.CostUSD)
		}
	})

	t.Run("cost recalculated even if already set", func(t *testing.T) {
		// CalculateCost 总是基于 tokens 和 pricing 重新计算，
		// 不保留旧值。调用方（fillMissingCost）在 CostUSD==0 时才调用。
		span := &domain.Span{
			Type:            domain.SpanTypeLLM,
			PromptTokens:    2000,
			CompletionTokens: 1000,
			CostUSD:         0.5,
		}
		// 2000/1000 * 0.01 + 1000/1000 * 0.02 = 0.02 + 0.02 = 0.04
		span.CalculateCost(domain.Pricing{PromptPrice: 0.01, CompletionPrice: 0.02})
		if span.CostUSD != 0.04 {
			t.Errorf("CostUSD = %f, want 0.04 (recalculated from tokens)", span.CostUSD)
		}
	})
}

// ===== Evaluation 实体测试 =====

func TestEvaluationScore(t *testing.T) {
	eval := &domain.Evaluation{
		Accuracy:       0.8,
		Completeness:   0.9,
		ToolSelection:  0.7,
		ReasoningDepth: 0.6,
		Helpfulness:    0.85,
	}

	t.Run("known dimension returns correct score", func(t *testing.T) {
		if eval.Score(domain.DimensionAccuracy) != 0.8 {
			t.Error("Accuracy score mismatch")
		}
		if eval.Score(domain.DimensionHelpfulness) != 0.85 {
			t.Error("Helpfulness score mismatch")
		}
	})

	t.Run("unknown dimension returns 0", func(t *testing.T) {
		if eval.Score("unknown_dimension") != 0 {
			t.Error("unknown dimension should return 0")
		}
	})
}

func TestEvaluationComputeOverallEqualWeights(t *testing.T) {
	eval := &domain.Evaluation{
		Accuracy:       1.0,
		Completeness:   0.0,
		ToolSelection:  1.0,
		ReasoningDepth: 0.0,
		Helpfulness:    1.0,
	}

	// 等权：(1.0 + 0.0 + 1.0 + 0.0 + 1.0) / 5 = 0.6
	overall := eval.ComputeOverall(nil)
	if overall != 0.6 {
		t.Errorf("ComputeOverall(nil) = %f, want 0.6", overall)
	}
}

func TestEvaluationComputeOverallCustomWeights(t *testing.T) {
	eval := &domain.Evaluation{
		Accuracy:       1.0,
		Completeness:   0.0,
		ToolSelection:  0.0,
		ReasoningDepth: 0.0,
		Helpfulness:    0.0,
	}

	weights := map[domain.EvaluationDimension]float32{
		domain.DimensionAccuracy: 1.0, // 只有 accuracy 有权重
	}
	overall := eval.ComputeOverall(weights)
	if overall != 1.0 {
		t.Errorf("ComputeOverall(custom) = %f, want 1.0", overall)
	}
}

func TestEvaluationComputeOverallZeroTotalWeight(t *testing.T) {
	eval := &domain.Evaluation{
		Accuracy:       1.0,
		Completeness:   1.0,
		ToolSelection:  1.0,
		ReasoningDepth: 1.0,
		Helpfulness:    1.0,
	}

	overall := eval.ComputeOverall(map[domain.EvaluationDimension]float32{})
	if overall != 0 {
		t.Errorf("ComputeOverall(empty weights) = %f, want 0", overall)
	}
}

// ===== 枚举与校验测试 =====

func TestAllDimensions(t *testing.T) {
	dims := domain.AllDimensions()
	expected := []domain.EvaluationDimension{
		domain.DimensionAccuracy,
		domain.DimensionCompleteness,
		domain.DimensionToolSelection,
		domain.DimensionReasoningDepth,
		domain.DimensionHelpfulness,
	}
	if len(dims) != len(expected) {
		t.Fatalf("AllDimensions returned %d, want %d", len(dims), len(expected))
	}
	for i, d := range expected {
		if dims[i] != d {
			t.Errorf("AllDimensions[%d] = %q, want %q", i, dims[i], d)
		}
	}
}

func TestAllCostDimensions(t *testing.T) {
	dims := domain.AllCostDimensions()
	if len(dims) != 6 {
		t.Fatalf("AllCostDimensions returned %d, want 6", len(dims))
	}
}

func TestValidSpanTypes(t *testing.T) {
	for _, st := range []domain.SpanType{
		domain.SpanTypeAgent, domain.SpanTypeLLM, domain.SpanTypeTool,
		domain.SpanTypeReasoning, domain.SpanTypeEvaluation,
	} {
		if !domain.IsValidSpanType(st) {
			t.Errorf("IsValidSpanType(%q) = false, want true", st)
		}
	}

	if domain.IsValidSpanType("nonexistent") {
		t.Error("IsValidSpanType('nonexistent') should be false")
	}
}

func TestValidSpanStatuses(t *testing.T) {
	for _, ss := range []domain.SpanStatus{
		domain.SpanStatusOK, domain.SpanStatusError, domain.SpanStatusTimeout,
	} {
		if !domain.IsValidSpanStatus(ss) {
			t.Errorf("IsValidSpanStatus(%q) = false, want true", ss)
		}
	}

	if domain.IsValidSpanStatus("nonexistent") {
		t.Error("IsValidSpanStatus('nonexistent') should be false")
	}
}

func TestValidCostDimensions(t *testing.T) {
	for _, d := range domain.AllCostDimensions() {
		if !domain.IsValidCostDimension(d) {
			t.Errorf("IsValidCostDimension(%q) = false, want true", d)
		}
	}

	if domain.IsValidCostDimension("invalid_dimension") {
		t.Error("IsValidCostDimension('invalid_dimension') should be false")
	}
}

func TestValidOrderBy(t *testing.T) {
	for _, col := range []string{"timestamp", "cost", "tokens", "latency", "start_time"} {
		if _, ok := domain.ValidOrderBy[col]; !ok {
			t.Errorf("ValidOrderBy missing %q", col)
		}
	}

	if _, ok := domain.ValidOrderBy["evil_column"]; ok {
		t.Error("ValidOrderBy should not contain 'evil_column'")
	}
}

func TestSentinelErrors(t *testing.T) {
	t.Run("ErrNotFound is detectable via errors.Is", func(t *testing.T) {
		err := domain.ErrNotFound
		if !errorsAreEqual(err, domain.ErrNotFound) {
			t.Error("ErrNotFound should match itself")
		}
	})

	t.Run("ErrInvalidInput is detectable", func(t *testing.T) {
		err := domain.ErrInvalidInput
		if !errorsAreEqual(err, domain.ErrInvalidInput) {
			t.Error("ErrInvalidInput should match itself")
		}
	})

	t.Run("different sentinels are not equal", func(t *testing.T) {
		if errorsAreEqual(domain.ErrNotFound, domain.ErrInvalidInput) {
			t.Error("different sentinels should not match")
		}
	})
}

// ===== TimeWindow 测试 =====

func TestTimeWindowSerialization(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	tw := domain.TimeWindow{From: from, To: to}

	if !tw.From.Equal(from) {
		t.Error("TimeWindow.From should preserve the input time")
	}
	if !tw.To.Equal(to) {
		t.Error("TimeWindow.To should preserve the input time")
	}
	if tw.From.After(tw.To) {
		t.Error("From should be before To")
	}
}

// ===== Pricing 测试 =====

func TestPricingExpiry(t *testing.T) {
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	t.Run("active pricing has nil expired_at", func(t *testing.T) {
		p := &domain.Pricing{
			Model:       "gpt-4o",
			PromptPrice: 0.01,
			EffectiveAt: now,
			ExpiredAt:   nil,
		}
		if p.ExpiredAt != nil {
			t.Error("active pricing should have nil ExpiredAt")
		}
	})

	t.Run("expired pricing has future expired_at", func(t *testing.T) {
		future := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
		p := &domain.Pricing{
			Model:       "gpt-4o",
			PromptPrice: 0.01,
			EffectiveAt: now,
			ExpiredAt:   &future,
		}
		if p.ExpiredAt == nil {
			t.Fatal("ExpiredAt should not be nil")
		}
		if p.ExpiredAt.Before(now) {
			t.Error("expired_at should be in the future")
		}
	})
}

// ===== FailureCluster 测试 =====

func TestFailureClusterCreation(t *testing.T) {
	cluster := &domain.FailureCluster{
		Name:         "timeout_error",
		TraceCount:   42,
		Percentage:   0.15,
		CommonPattern: `["connection timeout after 30s"]`,
		IsActive:      true,
		ExampleTraces: []string{"trace-1", "trace-2"},
		Metadata: map[string]any{
			"first_seen": "2025-01-01",
		},
	}

	if cluster.Name != "timeout_error" {
		t.Error("cluster name mismatch")
	}
	if cluster.TraceCount != 42 {
		t.Error("trace count mismatch")
	}
	if cluster.Percentage != 0.15 {
		t.Error("percentage mismatch")
	}
	if len(cluster.ExampleTraces) != 2 {
		t.Error("example traces length mismatch")
	}
	if cluster.Metadata["first_seen"] != "2025-01-01" {
		t.Error("metadata mismatch")
	}
}

// ===== TraceTree 测试 =====

func TestTraceTreeStructure(t *testing.T) {
	tree := &domain.TraceTree{
		TraceID:   "trace-123",
		SessionID: "session-456",
		UserID:    "user-789",
		Depth:     3,
	}

	if tree.TraceID != "trace-123" {
		t.Error("TraceID mismatch")
	}
	if tree.SessionID != "session-456" {
		t.Error("SessionID mismatch")
	}
	if tree.Depth != 3 {
		t.Error("Depth mismatch")
	}
}

// ===== VectorMatch 测试 =====

func TestVectorMatch(t *testing.T) {
	match := domain.VectorMatch{
		ID:    "vec-1",
		Score: 0.95,
		Metadata: map[string]any{
			"type": "error",
		},
	}

	if match.ID != "vec-1" {
		t.Error("ID mismatch")
	}
	if match.Score != 0.95 {
		t.Error("Score mismatch")
	}
	if match.Metadata["type"] != "error" {
		t.Error("Metadata mismatch")
	}
}

// ===== Judge 接口契约测试 =====

func TestJudgeInputOutput(t *testing.T) {
	input := &domain.JudgeInput{
		UserInput:   "What is the weather?",
		AgentOutput: "It is sunny today.",
		Metadata: map[string]any{
			"model": "gpt-4o",
		},
	}

	if input.UserInput == "" {
		t.Error("UserInput should not be empty")
	}
	if input.AgentOutput == "" {
		t.Error("AgentOutput should not be empty")
	}

	output := &domain.JudgeOutput{
		Scores: map[domain.EvaluationDimension]float32{
			domain.DimensionAccuracy: 0.9,
		},
		Rationale:  "The answer is accurate and helpful.",
		TokensUsed: 150,
	}

	if output.TokensUsed != 150 {
		t.Error("TokensUsed mismatch")
	}
	if len(output.Scores) != 1 {
		t.Error("scores map should have 1 entry")
	}
}

// ===== ABTest 实体测试 =====

func TestABTestCreation(t *testing.T) {
	test := &domain.ABTest{
		Name:             "new-prompt-test",
		AgentName:        "interview-agent",
		ControlVersion:   1,
		TreatmentVersion: 2,
		TrafficPercent:   50,
		Status:           domain.ABTestRunning,
	}

	if test.ControlVersion == test.TreatmentVersion {
		t.Error("control and treatment versions must differ")
	}
	if test.TrafficPercent < 1 || test.TrafficPercent > 100 {
		t.Error("traffic_percent must be between 1 and 100")
	}
}

// ===== Span.Validate 测试 =====

func TestSpanValidate(t *testing.T) {
	now := time.Now()

	t.Run("valid agent span", func(t *testing.T) {
		span := &domain.Span{
			ID: "span-1", TraceID: "trace-1", SessionID: "session-1",
			Type: domain.SpanTypeAgent, StartTime: now,
		}
		if err := span.Validate(); err != nil {
			t.Errorf("valid agent span should pass validation: %v", err)
		}
	})

	t.Run("valid llm span", func(t *testing.T) {
		span := &domain.Span{
			ID: "span-1", TraceID: "trace-1", SessionID: "session-1",
			Type: domain.SpanTypeLLM, Model: "gpt-4o",
			PromptTokens: 100, StartTime: now,
		}
		if err := span.Validate(); err != nil {
			t.Errorf("valid llm span should pass validation: %v", err)
		}
	})

	t.Run("valid tool span", func(t *testing.T) {
		span := &domain.Span{
			ID: "span-1", TraceID: "trace-1", SessionID: "session-1",
			Type: domain.SpanTypeTool, ToolName: "search",
			StartTime: now,
		}
		if err := span.Validate(); err != nil {
			t.Errorf("valid tool span should pass validation: %v", err)
		}
	})

	t.Run("missing id fails", func(t *testing.T) {
		span := &domain.Span{
			TraceID: "trace-1", SessionID: "session-1",
			Type: domain.SpanTypeAgent, StartTime: now,
		}
		if err := span.Validate(); err == nil {
			t.Error("expected error for missing id")
		}
	})

	t.Run("missing trace_id fails", func(t *testing.T) {
		span := &domain.Span{
			ID: "span-1", SessionID: "session-1",
			Type: domain.SpanTypeAgent, StartTime: now,
		}
		if err := span.Validate(); err == nil {
			t.Error("expected error for missing trace_id")
		}
	})

	t.Run("missing session_id fails", func(t *testing.T) {
		span := &domain.Span{
			ID: "span-1", TraceID: "trace-1",
			Type: domain.SpanTypeAgent, StartTime: now,
		}
		if err := span.Validate(); err == nil {
			t.Error("expected error for missing session_id")
		}
	})

	t.Run("llm span missing model fails", func(t *testing.T) {
		span := &domain.Span{
			ID: "span-1", TraceID: "trace-1", SessionID: "session-1",
			Type: domain.SpanTypeLLM, StartTime: now,
		}
		if err := span.Validate(); err == nil {
			t.Error("expected error for llm span without model")
		}
	})

	t.Run("tool span missing tool_name fails", func(t *testing.T) {
		span := &domain.Span{
			ID: "span-1", TraceID: "trace-1", SessionID: "session-1",
			Type: domain.SpanTypeTool, StartTime: now,
		}
		if err := span.Validate(); err == nil {
			t.Error("expected error for tool span without tool_name")
		}
	})
}

// ===== 辅助函数 =====

// errorsAreEqual 简单错误比较（因为我们没有使用 errors.Is 包装）。
func errorsAreEqual(a, b error) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Error() == b.Error()
}

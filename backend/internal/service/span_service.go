// Package service - Span 写入与查询服务。
package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/pkg/logger"
)

// SpanService 处理 Span 写入与查询。
//
// 关键职责：
//   - 异步批量写入：使用 worker pool + 缓冲队列
//   - 自动计算成本：如果 span 未设置 cost_usd 但有 tokens，按当前价格表计算
//   - 自动触发评估：根据配置采样触发 LLM-as-Judge
type SpanService struct {
	repo       domain.SpanRepository
	pricingRepo domain.PricingRepository
	logger     logger.Logger

	// 异步写入
	batchQueue  chan *domain.Span
	batchSize   int
	workerCount int
	workerWG    sync.WaitGroup

	closeOnce sync.Once
	closed    chan struct{}
	// stopped 优先于 select 竞态：closed 与 queue 同时就绪时 send 可能仍成功
	stopped atomic.Bool
}

// SpanServiceConfig Span 服务配置。
type SpanServiceConfig struct {
	BatchSize     int
	WorkerCount   int
	QueueSize     int
	FlushInterval int // 秒
}

// NewSpanService 创建服务实例。
//
// pricingRepo 可为 nil(测试场景);为 nil 时 fillMissingCost 跳过成本计算。
func NewSpanService(repo domain.SpanRepository, pricingRepo domain.PricingRepository, log logger.Logger) *SpanService {
	cfg := SpanServiceConfig{
		BatchSize:     100,
		WorkerCount:   2,
		QueueSize:     10000,
		FlushInterval: 5,
	}

	s := &SpanService{
		repo:        repo,
		pricingRepo: pricingRepo,
		logger:      log.WithFields(map[string]any{"component": "span_service"}),
		batchQueue:  make(chan *domain.Span, cfg.QueueSize),
		batchSize:   cfg.BatchSize,
		workerCount: cfg.WorkerCount,
		closed:      make(chan struct{}),
	}

	// 启动 worker
	for i := 0; i < cfg.WorkerCount; i++ {
		s.workerWG.Add(1)
		go s.batchWorker(i)
	}

	return s
}

// IngestSpans 处理一批 Span 入库。
//
// 流程：
//   1. 计算缺失的成本（基于价格表）
//   2. 批量异步写入 ClickHouse
func (s *SpanService) IngestSpans(ctx context.Context, spans []*domain.Span) error {
	if len(spans) == 0 {
		return nil
	}
	if s.stopped.Load() {
		return fmt.Errorf("service closed")
	}

	// 1. 计算缺失成本
	s.fillMissingCost(ctx, spans)

	// 2. 异步写入
	for _, span := range spans {
		if s.stopped.Load() {
			return fmt.Errorf("service closed")
		}
		select {
		case s.batchQueue <- span:
		case <-s.closed:
			return fmt.Errorf("service closed")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// fillMissingCost 自动计算缺失的成本。
func (s *SpanService) fillMissingCost(ctx context.Context, spans []*domain.Span) {
	if s.pricingRepo == nil {
		return
	}

	// 按 model 分组
	byModel := make(map[string][]*domain.Span)
	for _, span := range spans {
		if span.Type == domain.SpanTypeLLM && span.CostUSD == 0 && span.Model != "" {
			byModel[span.Model] = append(byModel[span.Model], span)
		}
	}

	// 一次性查询价格
	for model, ss := range byModel {
		pricing, err := s.pricingRepo.Get(ctx, model, ss[0].StartTime)
		if err != nil || pricing == nil {
			s.logger.Warnf("get pricing for %s: %v", model, err)
			continue
		}
		for _, span := range ss {
			span.CalculateCost(*pricing)
		}
	}
}

// GetByID 查询 Span。
func (s *SpanService) GetByID(ctx context.Context, id string) (*domain.Span, error) {
	return s.repo.GetByID(ctx, id)
}

// GetTraceTree 查询完整调用树。
func (s *SpanService) GetTraceTree(ctx context.Context, traceID string) (*domain.TraceTree, error) {
	return s.repo.GetTraceTree(ctx, traceID)
}

// ListBySession 查询会话。
func (s *SpanService) ListBySession(ctx context.Context, sessionID string, opts domain.ListOptions) ([]*domain.Span, error) {
	return s.repo.ListBySession(ctx, sessionID, opts)
}

// ListByUser 查询用户。
func (s *SpanService) ListByUser(ctx context.Context, userID string, opts domain.ListOptions) ([]*domain.Span, error) {
	return s.repo.ListByUser(ctx, userID, opts)
}

// ListByAgent 查询 Agent。
func (s *SpanService) ListByAgent(ctx context.Context, agentName string, opts domain.ListOptions) ([]*domain.Span, error) {
	return s.repo.ListByAgent(ctx, agentName, opts)
}

// ---------------------------------------------------------------------------
// 内部：批量写入 worker
// ---------------------------------------------------------------------------

func (s *SpanService) batchWorker(id int) {
	defer s.workerWG.Done()

	batch := make([]*domain.Span, 0, s.batchSize)
	flushTick := time.NewTicker(5 * time.Second)
	defer flushTick.Stop()

	for {
		select {
		case span := <-s.batchQueue:
			batch = append(batch, span)
			if len(batch) >= s.batchSize {
				s.flush(id, batch)
				batch = batch[:0]
			}
		case <-flushTick.C:
			if len(batch) > 0 {
				s.flush(id, batch)
				batch = batch[:0]
			}
		case <-s.closed:
			// 关闭后 drain 队列并最终 flush，避免 Shutdown 丢数
			for {
				select {
				case span := <-s.batchQueue:
					batch = append(batch, span)
					if len(batch) >= s.batchSize {
						s.flush(id, batch)
						batch = batch[:0]
					}
				default:
					if len(batch) > 0 {
						s.flush(id, batch)
					}
					return
				}
			}
		}
	}
}

func (s *SpanService) flush(workerID int, batch []*domain.Span) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.repo.BatchInsert(ctx, batch); err != nil {
		s.logger.Errorf("worker %d: flush %d spans: %v", workerID, len(batch), err)
		return
	}
	s.logger.Debugf("worker %d: flushed %d spans", workerID, len(batch))
}

// Shutdown 优雅关闭，等待所有 worker 完成。
func (s *SpanService) Shutdown(ctx context.Context) {
	s.closeOnce.Do(func() {
		// 先标记 stopped，再 close(closed)，避免 IngestSpans select 竞态仍入队
		s.stopped.Store(true)
		close(s.closed)
		done := make(chan struct{})
		go func() {
			s.workerWG.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
			// 超时仍返回；worker 可能在后台收尾
		}
	})
}
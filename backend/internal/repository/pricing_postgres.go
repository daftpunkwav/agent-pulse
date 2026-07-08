// Package repository - PostgreSQL Pricing 仓储实现。
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/agentpulse/backend/internal/domain"
	"github.com/agentpulse/backend/pkg/logger"
	"github.com/jackc/pgx/v5"
)

// PostgresPricingRepo PricingRepository 的 PostgreSQL 实现。
type PostgresPricingRepo struct {
	client *PostgresClient
	logger logger.Logger
}

// NewPostgresPricingRepo 创建仓储实例。
func NewPostgresPricingRepo(client *PostgresClient, log logger.Logger) *PostgresPricingRepo {
	return &PostgresPricingRepo{
		client: client,
		logger: log.WithFields(map[string]any{"component": "pricing_repo"}),
	}
}

type pricingRow struct {
	ModelName       string  `db:"model_name"`
	PromptPrice     float64 `db:"prompt_price"`
	CompletionPrice float64 `db:"completion_price"`
	Currency        string  `db:"currency"`
	EffectiveAt     time.Time `db:"effective_at"`
	ExpiredAt       *time.Time `db:"expired_at"`
}

func (r pricingRow) toDomain() *domain.Pricing {
	return &domain.Pricing{
		Model:           r.ModelName,
		PromptPrice:     r.PromptPrice,
		CompletionPrice: r.CompletionPrice,
		Currency:        r.Currency,
		EffectiveAt:     r.EffectiveAt,
		ExpiredAt:       r.ExpiredAt,
	}
}

// Get 查询指定时间点生效的价格。
//
// 时间匹配规则：effective_at <= at AND (expired_at IS NULL OR expired_at > at)
func (r *PostgresPricingRepo) Get(ctx context.Context, model string, at time.Time) (*domain.Pricing, error) {
	const query = `SELECT
		model_name, prompt_price, completion_price, currency,
		effective_at, expired_at
	FROM model_pricing
	WHERE model_name = $1
	  AND effective_at <= $2
	  AND (expired_at IS NULL OR expired_at > $2)
	ORDER BY effective_at DESC
	LIMIT 1`

	var (
		modelName       string
		promptPrice     float64
		completionPrice float64
		currency        string
		effectiveAt     time.Time
		expiredAt       *time.Time
	)

	err := r.client.Pool().QueryRow(ctx, query, model, at).Scan(
		&modelName, &promptPrice, &completionPrice, &currency,
		&effectiveAt, &expiredAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &domain.Pricing{
		Model:           modelName,
		PromptPrice:     promptPrice,
		CompletionPrice: completionPrice,
		Currency:        currency,
		EffectiveAt:     effectiveAt,
		ExpiredAt:       expiredAt,
	}, nil
}

// ListActive 列出所有当前生效的价格。
func (r *PostgresPricingRepo) ListActive(ctx context.Context) ([]*domain.Pricing, error) {
	const query = `SELECT
		model_name, prompt_price, completion_price, currency,
		effective_at, expired_at
	FROM model_pricing
	WHERE (expired_at IS NULL OR expired_at > NOW())
	ORDER BY model_name ASC`

	rows, err := r.client.Pool().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query pricing: %w", err)
	}
	defer rows.Close()

	var pricings []*domain.Pricing
	for rows.Next() {
		var (
			modelName       string
			promptPrice     float64
			completionPrice float64
			currency        string
			effectiveAt     time.Time
			expiredAt       *time.Time
		)
		if err := rows.Scan(
			&modelName, &promptPrice, &completionPrice, &currency,
			&effectiveAt, &expiredAt,
		); err != nil {
			return nil, err
		}
		pricings = append(pricings, &domain.Pricing{
			Model:           modelName,
			PromptPrice:     promptPrice,
			CompletionPrice: completionPrice,
			Currency:        currency,
			EffectiveAt:     effectiveAt,
			ExpiredAt:       expiredAt,
		})
	}

	return pricings, rows.Err()
}

// Upsert 插入或更新价格。
//
// 同一 model 在同一时间点只能有一条价格记录。
func (r *PostgresPricingRepo) Upsert(ctx context.Context, pricing *domain.Pricing) error {
	const query = `INSERT INTO model_pricing (
		model_name, prompt_price, completion_price, currency,
		effective_at, expired_at
	) VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (model_name, effective_at) DO UPDATE SET
		prompt_price = EXCLUDED.prompt_price,
		completion_price = EXCLUDED.completion_price,
		currency = EXCLUDED.currency,
		expired_at = EXCLUDED.expired_at`

	currency := pricing.Currency
	if currency == "" {
		currency = "USD"
	}

	_, err := r.client.Pool().Exec(ctx, query,
		pricing.Model,
		pricing.PromptPrice,
		pricing.CompletionPrice,
		currency,
		pricing.EffectiveAt,
		pricing.ExpiredAt,
	)

	if err != nil {
		return fmt.Errorf("upsert pricing: %w", err)
	}

	r.logger.Infof("upserted pricing for model=%s prompt=%f completion=%f",
		pricing.Model, pricing.PromptPrice, pricing.CompletionPrice)
	return nil
}
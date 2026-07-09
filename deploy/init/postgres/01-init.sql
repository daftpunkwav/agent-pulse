-- =============================================================================
-- AgentPulse PostgreSQL 初始化脚本
-- =============================================================================

-- 启用必要扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- ============================================================================
-- evaluations - LLM-as-Judge 评估结果
-- ============================================================================
CREATE TABLE IF NOT EXISTS evaluations (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    span_id           VARCHAR(64) NOT NULL,
    trace_id          VARCHAR(64) NOT NULL,
    session_id        VARCHAR(64) NOT NULL,
    user_id           VARCHAR(64) NOT NULL,
    agent_name        VARCHAR(64) NOT NULL,

    -- 五维评分（0-1）
    accuracy          DECIMAL(4, 3),
    completeness      DECIMAL(4, 3),
    tool_selection    DECIMAL(4, 3),
    reasoning_depth   DECIMAL(4, 3),
    helpfulness       DECIMAL(4, 3),
    overall           DECIMAL(4, 3),

    -- 评估元数据
    rationale         TEXT,
    judge_model       VARCHAR(64) NOT NULL,
    judge_prompt      TEXT,

    -- 触发方式
    trigger_type      VARCHAR(16) DEFAULT 'sync',  -- sync/sampled/offline/feedback
    sample_rate       DECIMAL(4, 3) DEFAULT 1.0,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_evaluations_session_id ON evaluations(session_id);
CREATE INDEX IF NOT EXISTS idx_evaluations_span_id ON evaluations(span_id);
CREATE INDEX IF NOT EXISTS idx_evaluations_user_id ON evaluations(user_id);
CREATE INDEX IF NOT EXISTS idx_evaluations_agent_name ON evaluations(agent_name);
CREATE INDEX IF NOT EXISTS idx_evaluations_created_at ON evaluations(created_at);

-- ============================================================================
-- failure_clusters - 失败模式聚类
-- ============================================================================
CREATE TABLE IF NOT EXISTS failure_clusters (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cluster_name    VARCHAR(128) NOT NULL,
    description     TEXT,
    trace_count     INTEGER NOT NULL DEFAULT 0,
    percentage      DECIMAL(5, 4),
    common_pattern  TEXT,
    suggestion      TEXT,
    example_traces  JSONB DEFAULT '[]'::jsonb,
    metadata        JSONB DEFAULT '{}'::jsonb,
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_failure_clusters_active ON failure_clusters(is_active);
CREATE INDEX IF NOT EXISTS idx_failure_clusters_created_at ON failure_clusters(created_at);

-- ============================================================================
-- harness_configs - Harness 配置版本
-- ============================================================================
CREATE TABLE IF NOT EXISTS harness_configs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_name      VARCHAR(64) NOT NULL,
    version         INTEGER NOT NULL,
    config_yaml     TEXT NOT NULL,
    config_hash     VARCHAR(64) NOT NULL,
    status          VARCHAR(16) DEFAULT 'archived',  -- production/canary/archived
    traffic_percent INTEGER DEFAULT 0 CHECK (traffic_percent >= 0 AND traffic_percent <= 100),
    notes           TEXT,
    created_by      VARCHAR(64),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    promoted_at     TIMESTAMPTZ,
    UNIQUE(agent_name, version)
);

CREATE INDEX IF NOT EXISTS idx_harness_configs_agent_name ON harness_configs(agent_name);
CREATE INDEX IF NOT EXISTS idx_harness_configs_status ON harness_configs(status);

-- ============================================================================
-- ab_tests - A/B 测试
-- ============================================================================
CREATE TABLE IF NOT EXISTS ab_tests (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name                VARCHAR(128) NOT NULL,
    agent_name          VARCHAR(64) NOT NULL,
    control_version     INTEGER NOT NULL,
    treatment_version   INTEGER NOT NULL,
    traffic_percent     INTEGER NOT NULL CHECK (traffic_percent > 0 AND traffic_percent <= 100),
    status              VARCHAR(16) DEFAULT 'running',  -- running/completed/aborted
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at            TIMESTAMPTZ,
    result              JSONB,  -- 胜出方、统计显著性、指标对比
    metadata            JSONB DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ab_tests_status ON ab_tests(status);
CREATE INDEX IF NOT EXISTS idx_ab_tests_agent_name ON ab_tests(agent_name);

-- ============================================================================
-- model_pricing - LLM 模型价格表（版本化）
-- ============================================================================
CREATE TABLE IF NOT EXISTS model_pricing (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    model_name      VARCHAR(64) NOT NULL,
    prompt_price    DECIMAL(12, 8) NOT NULL,  -- USD per 1k tokens
    completion_price DECIMAL(12, 8) NOT NULL,
    currency        VARCHAR(8) DEFAULT 'USD',
    effective_at    TIMESTAMPTZ NOT NULL,
    expired_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_pricing_model_name ON model_pricing(model_name);
CREATE INDEX IF NOT EXISTS idx_model_pricing_effective ON model_pricing(effective_at, expired_at);

-- 插入初始价格数据（2026-07 基准）
INSERT INTO model_pricing (model_name, prompt_price, completion_price, effective_at) VALUES
    ('gpt-4o', 0.0025, 0.01, '2024-07-01'),
    ('gpt-4o-mini', 0.00015, 0.0006, '2024-07-01'),
    ('gpt-4-turbo', 0.01, 0.03, '2024-04-01'),
    ('claude-3.5-sonnet', 0.003, 0.015, '2024-10-01'),
    ('claude-3-haiku', 0.00025, 0.00125, '2024-03-01'),
    ('deepseek-v3', 0.00014, 0.00028, '2024-12-01'),
    ('qwen2.5-72b', 0.0004, 0.0012, '2024-09-01')
ON CONFLICT DO NOTHING;

-- =============================================================================
-- AgentPulse ClickHouse 初始化脚本
-- =============================================================================
-- 此脚本在 ClickHouse 容器首次启动时自动执行
-- =============================================================================

-- 创建数据库（如不存在）
CREATE DATABASE IF NOT EXISTS agentpulse;

USE agentpulse;

-- ============================================================================
-- agent_spans - Agent 调用 Span 表
-- ============================================================================
-- 设计原则：
--   - 按月分区，便于数据生命周期管理
--   - 主键排序 (user_id, session_id, timestamp) 优化按用户/会话查询
--   - 使用 JSON 类型存储灵活的 attributes
-- ============================================================================
CREATE TABLE IF NOT EXISTS agent_spans (
    timestamp          DateTime64(9) CODEC(DoubleDelta, ZSTD(1)),
    trace_id           String CODEC(ZSTD(1)),
    span_id            String CODEC(ZSTD(1)),
    parent_span_id     String CODEC(ZSTD(1)),
    session_id         String CODEC(ZSTD(1)),
    user_id            String CODEC(ZSTD(1)),
    agent_name         LowCardinality(String) CODEC(ZSTD(1)),
    service_name       LowCardinality(String) CODEC(ZSTD(1)),
    environment        LowCardinality(String) DEFAULT 'production' CODEC(ZSTD(1)),

    -- Span 类型与基础信息
    span_type          LowCardinality(String) CODEC(ZSTD(1)),  -- llm/tool/reasoning/evaluation/agent
    span_name          String CODEC(ZSTD(1)),
    status             LowCardinality(String) DEFAULT 'ok' CODEC(ZSTD(1)),  -- ok/error/timeout

    -- LLM 专属字段
    model              LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),
    prompt_tokens      UInt32 DEFAULT 0 CODEC(ZSTD(1)),
    completion_tokens  UInt32 DEFAULT 0 CODEC(ZSTD(1)),
    total_tokens       UInt32 DEFAULT 0 CODEC(ZSTD(1)),
    cost_usd           Decimal(10, 6) DEFAULT 0 CODEC(ZSTD(1)),
    finish_reason      LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),

    -- 工具调用字段
    tool_name          LowCardinality(String) DEFAULT '' CODEC(ZSTD(1)),

    -- 推理步骤字段
    reasoning_step     UInt16 DEFAULT 0 CODEC(ZSTD(1)),

    -- 通用指标
    latency_ms         UInt32 DEFAULT 0 CODEC(ZSTD(1)),

    -- 输入输出与上下文
    input_preview      String DEFAULT '' CODEC(ZSTD(1)),
    output_preview     String DEFAULT '' CODEC(ZSTD(1)),
    error_message      String DEFAULT '' CODEC(ZSTD(1)),

    -- 灵活属性（JSON 字符串，应用层解析）
    attributes         String DEFAULT '{}' CODEC(ZSTD(1))
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (user_id, session_id, timestamp)
TTL toDateTime(timestamp) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- ============================================================================
-- 索引优化
-- ============================================================================
ALTER TABLE agent_spans ADD INDEX IF NOT EXISTS idx_agent_name agent_name TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE agent_spans ADD INDEX IF NOT EXISTS idx_model model TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE agent_spans ADD INDEX IF NOT EXISTS idx_span_type span_type TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE agent_spans ADD INDEX IF NOT EXISTS idx_status status TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE agent_spans ADD INDEX IF NOT EXISTS idx_tool_name tool_name TYPE bloom_filter(0.01) GRANULARITY 4;

-- ============================================================================
-- sessions - 会话汇总表（物化视图目标表，须先于 MV 创建）
-- ============================================================================
CREATE TABLE IF NOT EXISTS sessions (
    session_id           String,
    user_id              String,
    agent_name           LowCardinality(String),
    service_name         LowCardinality(String),
    started_at           DateTime64(9),
    ended_at             DateTime64(9),
    span_count           UInt32,
    total_prompt_tokens  UInt64,
    total_completion_tokens UInt64,
    total_tokens         UInt64,
    total_cost_usd       Decimal(18, 6),
    total_latency_ms     UInt64,
    error_count          UInt32,
    timeout_count        UInt32
)
ENGINE = SummingMergeTree()
ORDER BY (user_id, session_id)
PARTITION BY toYYYYMM(started_at)
TTL toDateTime(started_at) + INTERVAL 365 DAY;

-- ============================================================================
-- sessions_mv - 会话汇总物化视图
-- ============================================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS sessions_mv
TO sessions AS
SELECT
    session_id,
    user_id,
    agent_name,
    service_name,
    min(timestamp) AS started_at,
    max(timestamp) AS ended_at,
    count() AS span_count,
    sum(prompt_tokens) AS total_prompt_tokens,
    sum(completion_tokens) AS total_completion_tokens,
    sum(total_tokens) AS total_tokens,
    sum(cost_usd) AS total_cost_usd,
    sum(latency_ms) AS total_latency_ms,
    countIf(status = 'error') AS error_count,
    countIf(status = 'timeout') AS timeout_count
FROM agent_spans
WHERE session_id != ''
GROUP BY session_id, user_id, agent_name, service_name;
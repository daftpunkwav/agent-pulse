// Package config 提供 AgentPulse 配置加载。
//
// 基于 viper 实现，支持：
//   - YAML 配置文件（config.yaml）
//   - 环境变量覆盖（AGENTPULSE_* 前缀）
//   - 默认值兜底
//   - 配置验证
//
// 使用方式：
//   cfg, err := config.Load(".")
//   if err != nil { ... }
//   fmt.Println(cfg.Server.Port)
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// Config 是顶层配置结构。
type Config struct {
	Server      ServerConfig      `mapstructure:"server" validate:"required"`
	Log         LogConfig         `mapstructure:"log" validate:"required"`
	Auth        AuthConfig        `mapstructure:"auth" validate:"required"`
	ClickHouse  ClickHouseConfig  `mapstructure:"clickhouse" validate:"required"`
	Postgres    PostgresConfig    `mapstructure:"postgres" validate:"required"`
	Chroma      ChromaConfig      `mapstructure:"chroma"`
	Judge       JudgeConfig       `mapstructure:"judge" validate:"required"`
	OTLP        OTLPConfig        `mapstructure:"otlp" validate:"required"`
	Evaluation  EvaluationConfig  `mapstructure:"evaluation"`
}

// AuthConfig 鉴权配置。
//
// MVP 阶段使用配置文件白名单 + SHA-256 比对。
// Phase 2 计划: 迁移到 DB/API Key CRUD + JWT。
type AuthConfig struct {
	// Enabled 是否启用鉴权。生产环境必须为 true。
	Enabled bool `mapstructure:"enabled"`
	// APIKeys 允许的 API Key 明文列表(启动时 hash 后比对)。
	// 环境变量: AGENTPULSE_AUTH_API_KEYS=key1,key2,key3
	APIKeys []string `mapstructure:"api_keys"`
	// OTLPRequireKey OTLP 接收端是否要求 X-AgentPulse-Key。
	// 默认与 Enabled 相同,生产建议为 true。
	OTLPRequireKey *bool `mapstructure:"otlp_require_key"`
}

// ServerConfig HTTP 服务配置。
type ServerConfig struct {
	Host            string        `mapstructure:"host" validate:"required"`
	Port            int           `mapstructure:"port" validate:"required,min=1,max=65535"`
	Mode            string        `mapstructure:"mode" validate:"required,oneof=debug release test"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	// AllowedOrigins CORS 允许的来源列表(逗号分隔)。支持 * 表示全部。
	// 生产环境必须显式列出 web dashboard 的实际 origin。
	AllowedOrigins string `mapstructure:"allowed_origins"`
}

// LogConfig 日志配置。
type LogConfig struct {
	Level  string `mapstructure:"level" validate:"required,oneof=debug info warn error"`
	Format string `mapstructure:"format" validate:"required,oneof=json console"`
}

// ClickHouseConfig ClickHouse 连接配置。
type ClickHouseConfig struct {
	Host     string `mapstructure:"host" validate:"required"`
	Port     int    `mapstructure:"port" validate:"required,min=1,max=65535"`
	Database string `mapstructure:"database" validate:"required"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	// 连接池
	MaxOpenConns int `mapstructure:"max_open_conns"`
	MaxIdleConns int `mapstructure:"max_idle_conns"`
	// TLS
	TLSEnabled bool `mapstructure:"tls_enabled"`
}

// DSN 返回 ClickHouse DSN 字符串(密码字段包含敏感信息,仅在内部使用)。
func (c ClickHouseConfig) DSN() string {
	protocol := "tcp"
	if c.TLSEnabled {
		protocol = "tcp+tls"
	}
	auth := ""
	if c.Username != "" {
		auth = c.Username
		if c.Password != "" {
			auth += ":" + c.Password
		}
		auth += "@"
	}
	return fmt.Sprintf("%s://%s%s:%d/%s",
		protocol, auth, c.Host, c.Port, c.Database)
}

// MaskedDSN 返回 DSN 的脱敏形式,密码字段替换为 ****。
//
// 适用于日志输出、错误信息、健康检查等场景。
func (c ClickHouseConfig) MaskedDSN() string {
	protocol := "tcp"
	if c.TLSEnabled {
		protocol = "tcp+tls"
	}
	auth := ""
	if c.Username != "" {
		auth = c.Username
		if c.Password != "" {
			auth += ":****"
		}
		auth += "@"
	}
	return fmt.Sprintf("%s://%s%s:%d/%s",
		protocol, auth, c.Host, c.Port, c.Database)
}

// PostgresConfig PostgreSQL 连接配置。
type PostgresConfig struct {
	Host         string `mapstructure:"host" validate:"required"`
	Port         int    `mapstructure:"port" validate:"required,min=1,max=65535"`
	Database     string `mapstructure:"database" validate:"required"`
	Username     string `mapstructure:"username" validate:"required"`
	Password     string `mapstructure:"password"`
	SSLMode      string `mapstructure:"ssl_mode" validate:"omitempty,oneof=disable require verify-full verify-ca"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxLifetime  time.Duration `mapstructure:"max_lifetime"`
}

// DSN 返回 PostgreSQL DSN 字符串(pgx 格式,密码字段包含敏感信息,仅在内部使用)。
func (c PostgresConfig) DSN() string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.Host, c.Port, c.Database, c.Username, c.Password, sslMode,
	)
}

// MaskedDSN 返回 DSN 的脱敏形式,密码字段替换为 ****。
//
// 适用于日志输出、错误信息、健康检查等场景。
func (c PostgresConfig) MaskedDSN() string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=**** sslmode=%s",
		c.Host, c.Port, c.Database, c.Username, sslMode,
	)
}

// ChromaConfig Chroma 向量库配置。
type ChromaConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	APIKey   string `mapstructure:"api_key"`
	Tenant   string `mapstructure:"tenant"`
	Database string `mapstructure:"database"`
	// TLSEnabled 是否启用 TLS 连接（生产环境建议 true）。
	TLSEnabled bool `mapstructure:"tls_enabled"`
}

// BaseURL 返回 Chroma 服务 URL（根据 TLS 配置选择协议）。
func (c ChromaConfig) BaseURL() string {
	if c.Host == "" {
		return ""
	}
	protocol := "http"
	if c.TLSEnabled {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s:%d", protocol, c.Host, c.Port)
}

// JudgeConfig LLM-as-Judge 评估器配置。
type JudgeConfig struct {
	Model       string        `mapstructure:"model" validate:"required"`
	APIKey      string        `mapstructure:"api_key"`
	BaseURL     string        `mapstructure:"base_url"`
	Timeout     time.Duration `mapstructure:"timeout"`
	MaxRetries  int           `mapstructure:"max_retries"`
	Concurrency int           `mapstructure:"concurrency"`
}

// OTLPConfig OTLP 接收端配置。
type OTLPConfig struct {
	GRPCPort    int   `mapstructure:"grpc_port" validate:"required,min=1,max=65535"`
	HTTPPort    int   `mapstructure:"http_port" validate:"required,min=1,max=65535"`
	MaxBodySize int64 `mapstructure:"max_body_size"`
}

// EvaluationConfig 评估服务配置。
type EvaluationConfig struct {
	SampleRate        float64       `mapstructure:"sample_rate" validate:"min=0,max=1"`
	AsyncWorkers      int           `mapstructure:"async_workers"`
	AsyncQueueSize    int           `mapstructure:"async_queue_size"`
	DefaultDimensions []string      `mapstructure:"default_dimensions"`
	CacheTTL          time.Duration `mapstructure:"cache_ttl"`
}

// Load 从指定目录加载配置。
//
// 查找规则：
//   1. ./config.yaml
//   2. 环境变量（AGENTPULSE_* 前缀，覆盖文件配置）
//   3. 默认值
func Load(configDir string) (*Config, error) {
	v := viper.New()

	// 设置默认值
	setDefaults(v)

	// 配置文件
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir)
	v.AddConfigPath("./configs")
	v.AddConfigPath("/etc/agentpulse")

	// 环境变量
	v.SetEnvPrefix("AGENTPULSE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 读取文件（如果存在）
	if err := v.ReadInConfig(); err != nil {
		// 配置文件不存在不算错误，使用环境变量和默认值
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// 验证配置
	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// ---------------------------------------------------------------------------
// 内部辅助
// ---------------------------------------------------------------------------

func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	// 默认为 release: 防止忘改 mode 直接以 debug 启动暴露路由/性能下降。
	v.SetDefault("server.mode", "release")
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.shutdown_timeout", "30s")
	v.SetDefault("server.allowed_origins", "") // 留空 → 关闭 CORS;生产按需配置

	// Log
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	// Auth
	v.SetDefault("auth.enabled", false)         // 默认关闭(向后兼容 dev),生产必须显式 true
	v.SetDefault("auth.otlp_require_key", true)  // 默认 OTLP 强制要求 Key

	// ClickHouse
	v.SetDefault("clickhouse.host", "localhost")
	v.SetDefault("clickhouse.port", 9000)
	v.SetDefault("clickhouse.database", "agentpulse")
	v.SetDefault("clickhouse.username", "default")
	v.SetDefault("clickhouse.max_open_conns", 20)
	v.SetDefault("clickhouse.max_idle_conns", 10)

	// Postgres
	v.SetDefault("postgres.host", "localhost")
	v.SetDefault("postgres.port", 5432)
	v.SetDefault("postgres.database", "agentpulse")
	v.SetDefault("postgres.username", "agentpulse")
	v.SetDefault("postgres.password", "changeme")
	v.SetDefault("postgres.ssl_mode", "disable")
	v.SetDefault("postgres.max_open_conns", 20)
	v.SetDefault("postgres.max_idle_conns", 10)
	v.SetDefault("postgres.max_lifetime", "1h")

	// Chroma
	v.SetDefault("chroma.host", "localhost")
	v.SetDefault("chroma.port", 8000)
	v.SetDefault("chroma.tenant", "default_tenant")
	v.SetDefault("chroma.database", "default_database")
	v.SetDefault("chroma.tls_enabled", false)

	// Judge
	v.SetDefault("judge.model", "gpt-4o-mini")
	v.SetDefault("judge.base_url", "https://api.openai.com/v1")
	v.SetDefault("judge.timeout", "60s")
	v.SetDefault("judge.max_retries", 3)
	v.SetDefault("judge.concurrency", 5)

	// OTLP
	v.SetDefault("otlp.grpc_port", 4317)
	v.SetDefault("otlp.http_port", 4318)
	// 默认 10MB,防止单次请求把内存撑爆。
	v.SetDefault("otlp.max_body_size", int64(10<<20))

	// Evaluation
	v.SetDefault("evaluation.sample_rate", 1.0)
	v.SetDefault("evaluation.async_workers", 3)
	v.SetDefault("evaluation.async_queue_size", 1000)
	v.SetDefault("evaluation.default_dimensions", []string{
		"accuracy", "completeness", "tool_selection",
		"reasoning_depth", "helpfulness",
	})
	v.SetDefault("evaluation.cache_ttl", "24h")
}

func validate(cfg *Config) error {
	v := validator.New()

	// 自定义校验必须在 Struct 之前执行，以便覆盖 validator 默认错误信息。
	// 1. release 模式: 禁止默认密码 / 空白密码。
	if cfg.Server.Mode == "release" {
		if cfg.Postgres.Password == "" || cfg.Postgres.Password == "changeme" {
			return fmt.Errorf("postgres.password must be set to a non-default value in release mode")
		}
		if cfg.Judge.APIKey == "" {
			return fmt.Errorf("judge.api_key must be set in release mode")
		}
		if cfg.Auth.Enabled && len(cfg.APIKeysResolved()) == 0 {
			return fmt.Errorf("auth.enabled=true requires at least one API key (auth.api_keys)")
		}
	}

	// 2. auth.api_keys 至少 1 个时(无论 mode),长度必须 >= 16 字符。
	for _, k := range cfg.APIKeysResolved() {
		if len(k) < 16 {
			return fmt.Errorf("auth.api_keys contains a key shorter than 16 chars; refuse to accept weak keys")
		}
	}

	// 3. 执行 struct tag 验证（覆盖所有 required/min/max 等）。
	if err := v.Struct(cfg); err != nil {
		return err
	}

	return nil
}

// APIKeysResolved 合并配置与环境变量中的 API Keys。
//
// 环境变量 AGENTPULSE_AUTH_API_KEYS 用逗号分隔。
// 当环境变量存在时,优先使用环境变量(避免配置文件误提交泄露)。
func (c *Config) APIKeysResolved() []string {
	if v := strings.TrimSpace(os.Getenv("AGENTPULSE_AUTH_API_KEYS")); v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
		return out
	}
	return c.Auth.APIKeys
}
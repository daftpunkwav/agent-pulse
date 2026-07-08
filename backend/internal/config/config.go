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
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// Config 是顶层配置结构。
type Config struct {
	Server      ServerConfig      `mapstructure:"server" validate:"required"`
	Log         LogConfig         `mapstructure:"log" validate:"required"`
	ClickHouse  ClickHouseConfig  `mapstructure:"clickhouse" validate:"required"`
	Postgres    PostgresConfig    `mapstructure:"postgres" validate:"required"`
	Chroma      ChromaConfig      `mapstructure:"chroma"`
	Judge       JudgeConfig       `mapstructure:"judge" validate:"required"`
	OTLP        OTLPConfig        `mapstructure:"otlp" validate:"required"`
	Evaluation  EvaluationConfig  `mapstructure:"evaluation"`
}

// ServerConfig HTTP 服务配置。
type ServerConfig struct {
	Host            string        `mapstructure:"host" validate:"required"`
	Port            int           `mapstructure:"port" validate:"required,min=1,max=65535"`
	Mode            string        `mapstructure:"mode" validate:"required,oneof=debug release test"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
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

// DSN 返回 ClickHouse DSN 字符串。
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

// DSN 返回 PostgreSQL DSN 字符串（pgx 格式）。
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

// ChromaConfig Chroma 向量库配置。
type ChromaConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	APIKey   string `mapstructure:"api_key"`
	Tenant   string `mapstructure:"tenant"`
	Database string `mapstructure:"database"`
}

// BaseURL 返回 Chroma 服务 URL。
func (c ChromaConfig) BaseURL() string {
	if c.Host == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", c.Host, c.Port)
}

// JudgeConfig LLM-as-Judge 评估器配置。
type JudgeConfig struct {
	Model       string        `mapstructure:"model" validate:"required"`
	APIKey      string        `mapstructure:"api_key" validate:"required"`
	BaseURL     string        `mapstructure:"base_url"`
	Timeout     time.Duration `mapstructure:"timeout"`
	MaxRetries  int           `mapstructure:"max_retries"`
	Concurrency int           `mapstructure:"concurrency"`
}

// OTLPConfig OTLP 接收端配置。
type OTLPConfig struct {
	GRPCPort int `mapstructure:"grpc_port" validate:"required,min=1,max=65535"`
	HTTPPort int `mapstructure:"http_port" validate:"required,min=1,max=65535"`
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
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.shutdown_timeout", "30s")

	// Log
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

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

	// Judge
	v.SetDefault("judge.model", "gpt-4o-mini")
	v.SetDefault("judge.base_url", "https://api.openai.com/v1")
	v.SetDefault("judge.timeout", "60s")
	v.SetDefault("judge.max_retries", 3)
	v.SetDefault("judge.concurrency", 5)

	// OTLP
	v.SetDefault("otlp.grpc_port", 4317)
	v.SetDefault("otlp.http_port", 4318)

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
	return v.Struct(cfg)
}
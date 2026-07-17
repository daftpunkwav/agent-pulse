// Package config_test 验证配置加载与校验逻辑。
package config_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/agentpulse/backend/internal/config"
)

// assertErrorContains 确认 err 非 nil 且包含子串。
func assertErrorContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("expected error containing %q, got %q", substr, err.Error())
	}
}

func TestLoadDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	os.Unsetenv("AGENTPULSE_AUTH_API_KEYS")
	// 使用 debug 模式避免 release 模式的严格密码验证
	os.Setenv("AGENTPULSE_SERVER_MODE", "debug")
	os.Setenv("AGENTPULSE_JUDGE_API_KEY", "sk-test-key-1234567890abcdef")
	defer func() {
		os.Unsetenv("AGENTPULSE_SERVER_MODE")
		os.Unsetenv("AGENTPULSE_JUDGE_API_KEY")
	}()

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}

	assertEqual(t, cfg.Server.Host, "0.0.0.0", "Server.Host")
	assertEqual(t, cfg.Server.Port, 8080, "Server.Port")
	// Mode 被环境变量 AGENTPULSE_SERVER_MODE=debug 覆盖
	assertEqual(t, cfg.Server.Mode, "debug", "Server.Mode")
	assertEqual(t, cfg.Auth.Enabled, false, "Auth.Enabled")
	assertEqual(t, cfg.Log.Level, "info", "Log.Level")
	assertEqual(t, cfg.Judge.Model, "gpt-4o-mini", "Judge.Model")
	assertEqual(t, cfg.Judge.BaseURL, "https://api.openai.com/v1", "Judge.BaseURL")
	assertEqual(t, cfg.Evaluation.SampleRate, 1.0, "Evaluation.SampleRate")
	assertEqual(t, cfg.Evaluation.AsyncWorkers, 3, "Evaluation.AsyncWorkers")
}

func TestLoadWithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte(`
server:
  host: "127.0.0.1"
  port: 9090
  mode: "debug"

log:
  level: "debug"
  format: "console"

auth:
  enabled: true
  api_keys: ["test-key-1234567890abcdef"]

clickhouse:
  host: "localhost"
  port: 9000
  database: "agentpulse"
  username: "default"
  password: "clickhouse_dev"

postgres:
  host: "localhost"
  port: 5432
  database: "agentpulse"
  username: "agentpulse"
  password: "postgres_dev"

judge:
  model: "gpt-4o"
  api_key: "sk-test-key-1234567890abcdef"
  base_url: "https://api.openai.com/v1"

otlp:
  grpc_port: 4317
  http_port: 4318

evaluation:
  sample_rate: 0.5
  async_workers: 2
  async_queue_size: 500
`)
	if err := os.WriteFile(tmpDir+"/config.yaml", content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, cfg.Server.Host, "127.0.0.1", "Server.Host")
	assertEqual(t, cfg.Server.Port, 9090, "Server.Port")
	assertEqual(t, cfg.Server.Mode, "debug", "Server.Mode")
	assertEqual(t, cfg.Auth.Enabled, true, "Auth.Enabled")
	assertEqual(t, cfg.Judge.Model, "gpt-4o", "Judge.Model")
	assertEqual(t, cfg.Evaluation.SampleRate, 0.5, "Evaluation.SampleRate")
	assertEqual(t, cfg.Evaluation.AsyncWorkers, 2, "Evaluation.AsyncWorkers")
	assertEqual(t, cfg.Evaluation.AsyncQueueSize, 500, "Evaluation.AsyncQueueSize")
}

func TestValidateReleaseModeRequiresNonDefaultPassword(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte(`
server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"

log:
  level: "info"
  format: "json"

auth:
  enabled: true
  api_keys: ["test-key-1234567890abcdef"]

clickhouse:
  host: "localhost"
  port: 9000
  database: "agentpulse"
  username: "default"

postgres:
  host: "localhost"
  port: 5432
  database: "agentpulse"
  username: "agentpulse"
  password: "changeme"

judge:
  model: "gpt-4o"
  api_key: "sk-test-key-1234567890abcdef"

otlp:
  grpc_port: 4317
  http_port: 4318
`)
	if err := os.WriteFile(tmpDir+"/config.yaml", content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.Load(tmpDir)
	assertErrorContains(t, err, "postgres.password must be set")
}

func TestValidateReleaseModeRequiresJudgeAPIKey(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte(`
server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"

log:
  level: "info"
  format: "json"

auth:
  enabled: false

clickhouse:
  host: "localhost"
  port: 9000
  database: "agentpulse"
  username: "default"

postgres:
  host: "localhost"
  port: 5432
  database: "agentpulse"
  username: "agentpulse"
  password: "strong-password-123"

judge:
  model: "gpt-4o"
  api_key: ""

otlp:
  grpc_port: 4317
  http_port: 4318
`)
	if err := os.WriteFile(tmpDir+"/config.yaml", content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.Load(tmpDir)
	assertErrorContains(t, err, "judge.api_key must be set")
}

func TestValidateAuthEnabledRequiresAPIKey(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte(`
server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"

log:
  level: "info"
  format: "json"

auth:
  enabled: true
  api_keys: []

clickhouse:
  host: "localhost"
  port: 9000
  database: "agentpulse"
  username: "default"

postgres:
  host: "localhost"
  port: 5432
  database: "agentpulse"
  username: "agentpulse"
  password: "strong-password-123"

judge:
  model: "gpt-4o"
  api_key: "sk-test-key-1234567890abcdef"

otlp:
  grpc_port: 4317
  http_port: 4318
`)
	if err := os.WriteFile(tmpDir+"/config.yaml", content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.Load(tmpDir)
	assertErrorContains(t, err, "auth.enabled=true requires at least one API key")
}

func TestValidateReleaseRequiresAuthEnabled(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte(`
server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"

log:
  level: "info"
  format: "json"

auth:
  enabled: false

clickhouse:
  host: "localhost"
  port: 9000
  database: "agentpulse"
  username: "default"

postgres:
  host: "localhost"
  port: 5432
  database: "agentpulse"
  username: "agentpulse"
  password: "strong-password-123"

judge:
  model: "gpt-4o"
  api_key: "sk-test-key-1234567890abcdef"

otlp:
  grpc_port: 4317
  http_port: 4318
`)
	if err := os.WriteFile(tmpDir+"/config.yaml", content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.Load(tmpDir)
	assertErrorContains(t, err, "auth.enabled must be true in release mode")
}

func TestValidateShortAPIKeyRejected(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte(`
server:
  host: "0.0.0.0"
  port: 8080
  mode: "debug"

log:
  level: "info"
  format: "json"

auth:
  enabled: true
  api_keys: ["short"]

clickhouse:
  host: "localhost"
  port: 9000
  database: "agentpulse"
  username: "default"

postgres:
  host: "localhost"
  port: 5432
  database: "agentpulse"
  username: "agentpulse"
  password: "postgres_dev"

judge:
  model: "gpt-4o"
  api_key: "sk-test-key-1234567890abcdef"

otlp:
  grpc_port: 4317
  http_port: 4318
`)
	if err := os.WriteFile(tmpDir+"/config.yaml", content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.Load(tmpDir)
	assertErrorContains(t, err, "shorter than 16 chars")
}

func TestAPIKeysResolvedFromEnv(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte(`
server:
  host: "0.0.0.0"
  port: 8080
  mode: "debug"

log:
  level: "info"
  format: "json"

auth:
  enabled: true
  api_keys: ["file-key-1234567890abcdef"]

clickhouse:
  host: "localhost"
  port: 9000
  database: "agentpulse"
  username: "default"

postgres:
  host: "localhost"
  port: 5432
  database: "agentpulse"
  username: "agentpulse"
  password: "postgres_dev"

judge:
  model: "gpt-4o"
  api_key: "sk-test-key-1234567890abcdef"

otlp:
  grpc_port: 4317
  http_port: 4318
`)
	if err := os.WriteFile(tmpDir+"/config.yaml", content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	os.Setenv("AGENTPULSE_AUTH_API_KEYS", "env-key-one-12345678,env-key-two-12345678")
	defer os.Unsetenv("AGENTPULSE_AUTH_API_KEYS")

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keys := cfg.APIKeysResolved()
	if len(keys) != 2 || keys[0] != "env-key-one-12345678" || keys[1] != "env-key-two-12345678" {
		t.Errorf("APIKeysResolved = %v, want [env-key-one-12345678 env-key-two-12345678]", keys)
	}
}

func TestAPIKeysResolvedFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.Unsetenv("AGENTPULSE_AUTH_API_KEYS")

	content := []byte(`
server:
  host: "0.0.0.0"
  port: 8080
  mode: "debug"

log:
  level: "info"
  format: "json"

auth:
  enabled: true
  api_keys: ["file-only-key-12345678"]

clickhouse:
  host: "localhost"
  port: 9000
  database: "agentpulse"
  username: "default"

postgres:
  host: "localhost"
  port: 5432
  database: "agentpulse"
  username: "agentpulse"
  password: "postgres_dev"

judge:
  model: "gpt-4o"
  api_key: "sk-test-key-1234567890abcdef"

otlp:
  grpc_port: 4317
  http_port: 4318
`)
	if err := os.WriteFile(tmpDir+"/config.yaml", content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keys := cfg.APIKeysResolved()
	if len(keys) != 1 || keys[0] != "file-only-key-12345678" {
		t.Errorf("APIKeysResolved = %v, want [file-only-key-12345678]", keys)
	}
}

func TestValidateAPIKey(t *testing.T) {
	allowed := []string{"my-secret-key-12345678", "another-key-12345678"}

	t.Run("valid key accepted", func(t *testing.T) {
		if !config.ValidateAPIKey(allowed, true, "my-secret-key-12345678") {
			t.Error("expected valid key to be accepted")
		}
	})

	t.Run("invalid key rejected", func(t *testing.T) {
		if config.ValidateAPIKey(allowed, true, "wrong-key") {
			t.Error("expected invalid key to be rejected")
		}
	})

	t.Run("requireKey=false bypasses check", func(t *testing.T) {
		if !config.ValidateAPIKey(allowed, false, "") {
			t.Error("expected empty key to pass when requireKey=false")
		}
		if !config.ValidateAPIKey(nil, false, "anything") {
			t.Error("expected any key to pass when requireKey=false")
		}
	})

	t.Run("empty allowed list rejects all", func(t *testing.T) {
		if config.ValidateAPIKey(nil, true, "any-key") {
			t.Error("expected nil allowed list to reject")
		}
		if config.ValidateAPIKey([]string{}, true, "any-key") {
			t.Error("expected empty allowed list to reject")
		}
	})
}

func TestClickHouseConfigDSN(t *testing.T) {
	t.Run("without auth", func(t *testing.T) {
		cfg := config.ClickHouseConfig{
			Host: "localhost", Port: 9000, Database: "agentpulse",
		}
		assertEqual(t, cfg.DSN(), "tcp://localhost:9000/agentpulse", "DSN")
	})

	t.Run("with auth no TLS", func(t *testing.T) {
		cfg := config.ClickHouseConfig{
			Host: "ch.example.com", Port: 9000, Database: "agentpulse",
			Username: "admin", Password: "secret",
		}
		expected := "tcp://admin:secret@ch.example.com:9000/agentpulse"
		assertEqual(t, cfg.DSN(), expected, "DSN")
	})

	t.Run("with TLS enabled", func(t *testing.T) {
		cfg := config.ClickHouseConfig{
			Host: "ch.example.com", Port: 9440, Database: "agentpulse",
			Username: "admin", Password: "secret", TLSEnabled: true,
		}
		expected := "tcp+tls://admin:secret@ch.example.com:9440/agentpulse"
		assertEqual(t, cfg.DSN(), expected, "DSN")
	})

	t.Run("masked DSN hides password", func(t *testing.T) {
		cfg := config.ClickHouseConfig{
			Host: "ch.example.com", Port: 9000, Database: "agentpulse",
			Username: "admin", Password: "super-secret",
		}
		masked := cfg.MaskedDSN()
		if !strings.Contains(masked, "****") {
			t.Errorf("masked DSN should contain ****: %s", masked)
		}
		if strings.Contains(masked, "super-secret") {
			t.Errorf("masked DSN should not contain password: %s", masked)
		}
	})
}

func TestPostgresConfigDSN(t *testing.T) {
	t.Run("basic DSN", func(t *testing.T) {
		cfg := config.PostgresConfig{
			Host: "localhost", Port: 5432, Database: "agentpulse",
			Username: "agentpulse", Password: "secret", SSLMode: "disable",
		}
		dsn := cfg.DSN()
		for _, part := range []string{
			"host=localhost", "port=5432", "dbname=agentpulse",
			"user=agentpulse", "password=secret", "sslmode=disable",
		} {
			if !strings.Contains(dsn, part) {
				t.Errorf("DSN missing %q: %s", part, dsn)
			}
		}
	})

	t.Run("default SSL mode is disable", func(t *testing.T) {
		cfg := config.PostgresConfig{
			Host: "localhost", Port: 5432, Database: "test",
			Username: "user", Password: "pw",
		}
		if !strings.Contains(cfg.DSN(), "sslmode=disable") {
			t.Errorf("expected default sslmode=disable, got: %s", cfg.DSN())
		}
	})

	t.Run("masked DSN hides password", func(t *testing.T) {
		cfg := config.PostgresConfig{
			Host: "pg.example.com", Port: 5432, Database: "prod",
			Username: "admin", Password: "top-secret", SSLMode: "require",
		}
		masked := cfg.MaskedDSN()
		if !strings.Contains(masked, "password=****") {
			t.Errorf("masked DSN should contain password=****: %s", masked)
		}
		if strings.Contains(masked, "top-secret") {
			t.Errorf("masked DSN should not contain password: %s", masked)
		}
		if !strings.Contains(masked, "sslmode=require") {
			t.Errorf("masked DSN should preserve sslmode: %s", masked)
		}
	})
}

func TestChromaConfigBaseURL(t *testing.T) {
	t.Run("empty host returns empty", func(t *testing.T) {
		cfg := config.ChromaConfig{}
		if cfg.BaseURL() != "" {
			t.Errorf("expected empty string, got %q", cfg.BaseURL())
		}
	})

	t.Run("HTTP default", func(t *testing.T) {
		cfg := config.ChromaConfig{Host: "localhost", Port: 8000}
		if cfg.BaseURL() != "http://localhost:8000" {
			t.Errorf("BaseURL = %q", cfg.BaseURL())
		}
	})

	t.Run("HTTPS with TLS enabled", func(t *testing.T) {
		cfg := config.ChromaConfig{Host: "chroma.example.com", Port: 443, TLSEnabled: true}
		if cfg.BaseURL() != "https://chroma.example.com:443" {
			t.Errorf("BaseURL = %q", cfg.BaseURL())
		}
	})
}

func TestConfigYAMLMissingUsesDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	// 使用 debug 模式避免 release 模式的严格密码验证
	os.Setenv("AGENTPULSE_SERVER_MODE", "debug")
	os.Setenv("AGENTPULSE_JUDGE_API_KEY", "sk-test-key-1234567890abcdef")
	defer func() {
		os.Unsetenv("AGENTPULSE_SERVER_MODE")
		os.Unsetenv("AGENTPULSE_JUDGE_API_KEY")
	}()

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}

	assertEqual(t, cfg.Server.Mode, "debug", "Server.Mode")
	assertEqual(t, cfg.Server.Port, 8080, "Server.Port")
}

func TestConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte(`server: [unbalanced bracket`)
	if err := os.WriteFile(tmpDir+"/config.yaml", content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.Load(tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestConfigValidationDebugModeAllowsDefaultPassword(t *testing.T) {
	tmpDir := t.TempDir()

	content := []byte(`
server:
  host: "0.0.0.0"
  port: 8080
  mode: "debug"

log:
  level: "info"
  format: "json"

auth:
  enabled: false

clickhouse:
  host: "localhost"
  port: 9000
  database: "agentpulse"
  username: "default"

postgres:
  host: "localhost"
  port: 5432
  database: "agentpulse"
  username: "agentpulse"
  password: "changeme"

judge:
  model: "gpt-4o"
  api_key: "sk-test-key-1234567890abcdef"

otlp:
  grpc_port: 4317
  http_port: 4318
`)
	if err := os.WriteFile(tmpDir+"/config.yaml", content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.Load(tmpDir)
	if err != nil {
		t.Errorf("debug mode should allow default password, got error: %v", err)
	}
}

// assertEqual 通用断言（支持 float64 精度比较）。
func assertEqual(t *testing.T, got, want interface{}, label string) {
	t.Helper()
	if !valuesEqual(got, want) {
		t.Errorf("%s = %v (type %T), want %v (type %T)", label, got, got, want, want)
	}
}

// valuesEqual 处理浮点比较和切片比较。
func valuesEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case []string:
		bv, ok := b.([]string)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

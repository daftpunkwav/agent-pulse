// 单元测试：release 模式强制鉴权
package unit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentpulse/backend/internal/config"
)

func writeCfg(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseModeRequiresAuthEnabled(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, `
server:
  mode: release
  port: 8080
auth:
  enabled: false
postgres:
  password: strong-password-12345
judge:
  api_key: sk-test-key-1234567890abcdef
`)
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "auth.enabled must be true") {
		t.Fatalf("expected auth.enabled error, got %v", err)
	}
}

func TestReleaseModeRequiresAPIKey(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, `
server:
  mode: release
  port: 8080
auth:
  enabled: true
  api_keys: []
postgres:
  password: strong-password-12345
judge:
  api_key: sk-test-key-1234567890abcdef
`)
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected API key error, got %v", err)
	}
}

func TestDebugModeAllowsAuthDisabled(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, `
server:
  mode: debug
  port: 8080
auth:
  enabled: false
postgres:
  password: changeme
judge:
  api_key: ""
`)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("debug load: %v", err)
	}
	if cfg.Auth.Enabled {
		t.Fatal("expected auth disabled in debug")
	}
}

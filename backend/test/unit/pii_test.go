// 单元测试：PII 脱敏
package unit_test

import (
	"strings"
	"testing"

	"github.com/agentpulse/backend/internal/pii"
)

func TestRedactEmail(t *testing.T) {
	in := "contact me at alice@example.com please"
	out := pii.Redact(in)
	if strings.Contains(out, "alice@example.com") {
		t.Fatalf("email not redacted: %s", out)
	}
	if !strings.Contains(out, "[REDACTED_EMAIL]") {
		t.Fatalf("missing redaction marker: %s", out)
	}
}

func TestRedactPhoneCN(t *testing.T) {
	in := "call 13800138000 now"
	out := pii.Redact(in)
	if strings.Contains(out, "13800138000") {
		t.Fatalf("phone not redacted: %s", out)
	}
}

func TestRedactJWT(t *testing.T) {
	// 三段式 JWT 样例（非真实密钥）
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signaturepart"
	out := pii.Redact("token=" + jwt)
	if strings.Contains(out, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Fatalf("jwt not redacted: %s", out)
	}
}

func TestRedactEmpty(t *testing.T) {
	if pii.Redact("") != "" {
		t.Fatal("empty should stay empty")
	}
}

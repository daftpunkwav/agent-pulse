// Package pii 提供通用 PII 脱敏能力。
//
// 在送往外部 LLM Judge、导出或日志前剥离常见敏感模式。
// 这是纵深防御层，并非 100% PII 防火墙。
package pii

import "regexp"

type pattern struct {
	name    string
	pattern *regexp.Regexp
	repl    string
}

var patterns = []pattern{
	{
		name:    "email",
		pattern: regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
		repl:    "[REDACTED_EMAIL]",
	},
	{
		name:    "phone_cn",
		pattern: regexp.MustCompile(`\b1[3-9]\d{9}\b`),
		repl:    "[REDACTED_PHONE]",
	},
	{
		name:    "credit_card",
		pattern: regexp.MustCompile(`\b(?:\d[ \-]?){13,19}\b`),
		repl:    "[REDACTED_CARD]",
	},
	{
		name:    "id_card_cn",
		pattern: regexp.MustCompile(`\b\d{17}[\dXx]\b`),
		repl:    "[REDACTED_ID]",
	},
	{
		name:    "bearer_token",
		pattern: regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]{8,}`),
		repl:    "Bearer [REDACTED_TOKEN]",
	},
	{
		name:    "api_key_like",
		pattern: regexp.MustCompile(`(?i)(api[_\-]?key|apikey|secret|token|password)\s*[:=]\s*["']?([A-Za-z0-9._\-]{8,})["']?`),
		repl:    "$1: [REDACTED]",
	},
	{
		name:    "jwt",
		pattern: regexp.MustCompile(`eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}`),
		repl:    "[REDACTED_JWT]",
	},
}

// Redact 剥离字符串中的常见 PII 模式。
func Redact(s string) string {
	if s == "" {
		return s
	}
	out := s
	for _, p := range patterns {
		out = p.pattern.ReplaceAllString(out, p.repl)
	}
	return out
}

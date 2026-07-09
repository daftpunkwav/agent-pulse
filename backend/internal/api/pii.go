// Package api - PII redaction helpers.
//
// Used to strip common PII from input/output before sending to external
// LLM APIs, exporting data, or logging.
package api

import "regexp"

// piiPattern is a redaction rule applied in order.
type piiPattern struct {
	name    string
	pattern *regexp.Regexp
	repl    string
}

var piiPatterns = []piiPattern{
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

// RedactPII scrubs common PII patterns from a string.
//
// This is a defense-in-depth layer, not a 100% PII firewall. True protection
// requires redacting at SDK ingress and validating at storage time.
func RedactPII(s string) string {
	if s == "" {
		return s
	}
	out := s
	for _, p := range piiPatterns {
		out = p.pattern.ReplaceAllString(out, p.repl)
	}
	return out
}

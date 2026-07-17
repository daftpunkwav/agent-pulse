// Package api - PII redaction helpers.
//
// Used to strip common PII from input/output before sending to external
// LLM APIs, exporting data, or logging.
package api

import "github.com/agentpulse/backend/internal/pii"

// RedactPII scrubs common PII patterns from a string.
//
// This is a defense-in-depth layer, not a 100% PII firewall. True protection
// requires redacting at SDK ingress and validating at storage time.
func RedactPII(s string) string {
	return pii.Redact(s)
}

// Package config - API Key 鉴权辅助。
//
// 提供独立于 api/collector 包的 API Key 校验函数,
// 避免 collector 与 api 之间的循环 import。
package config

import (
	"crypto/sha256"
	"crypto/subtle"
)

// ValidateAPIKey 校验 API Key 是否在白名单中。
//
// requireKey 为 true 时强制校验,false 时放行(dev 环境)。
// allowed 为空 + requireKey=true 时零信任拒绝。
func ValidateAPIKey(allowed []string, requireKey bool, apiKey string) bool {
	if !requireKey {
		return true
	}
	if len(allowed) == 0 {
		return false
	}
	sum := sha256.Sum256([]byte(apiKey))
	for _, k := range allowed {
		kSum := sha256.Sum256([]byte(k))
		if subtle.ConstantTimeCompare(sum[:], kSum[:]) == 1 {
			return true
		}
	}
	return false
}

package providers

import (
	"encoding/json"
	"strconv"
)

// ReadTokenNumber extracts the first present numeric field among keys,
// tolerating float/int/json.Number/string encodings across provider payloads.
func ReadTokenNumber(m map[string]interface{}, keys ...string) (int, bool) {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			return int(n), true
		case int:
			return n, true
		case int64:
			return int(n), true
		case json.Number:
			if parsed, err := n.Int64(); err == nil {
				return int(parsed), true
			}
		case string:
			if parsed, err := strconv.Atoi(n); err == nil {
				return parsed, true
			}
			if parsed, err := strconv.ParseFloat(n, 64); err == nil {
				return int(parsed), true
			}
		}
	}
	return 0, false
}

// OpenAIUsage is the usage block of an OpenAI-compatible response, including
// the prompt-cache split where the upstream reports one. Vendors spell that
// split three different ways: OpenAI nests cached_tokens under
// prompt_tokens_details, Anthropic-flavoured gateways use the
// cache_read/cache_creation_input_tokens pair, and some report both.
type OpenAIUsage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// CacheSplit returns the cached-read and cache-write token counts, and whether
// the upstream reported either. The "reported" flag matters: a provider that
// stays silent about caching is not the same as one reporting zero hits.
func (u OpenAIUsage) CacheSplit() (read, write int, reported bool) {
	read = u.CacheReadInputTokens
	if read == 0 {
		read = u.PromptTokensDetails.CachedTokens
	}
	write = u.CacheCreationInputTokens
	return read, write, read > 0 || write > 0
}

// ReportCacheUsage forwards a usage block's cache split to the callback when
// the upstream reported one.
func ReportCacheUsage(callback *StreamCallback, u OpenAIUsage) {
	if callback == nil || callback.OnCacheUsage == nil {
		return
	}
	if read, write, ok := u.CacheSplit(); ok {
		callback.OnCacheUsage(read, write)
	}
}

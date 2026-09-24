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

// CacheSplitFromMap pulls the prompt-cache split out of a decoded usage map,
// for providers that parse usage dynamically. It reads both the flat keys and
// the nested prompt_tokens_details / input_tokens_details objects, so it works
// against OpenAI-, Anthropic- and Responses-shaped usage blocks alike.
func CacheSplitFromMap(usage map[string]interface{}) (read, write int, reported bool) {
	if usage == nil {
		return 0, 0, false
	}
	if v, ok := ReadTokenNumber(usage, "cacheReadInputTokens", "cache_read_input_tokens", "cached_tokens", "cachedTokens"); ok {
		read = v
	}
	if v, ok := ReadTokenNumber(usage, "cacheWriteInputTokens", "cache_write_input_tokens", "cacheCreationInputTokens", "cache_creation_input_tokens"); ok {
		write = v
	}
	if read == 0 {
		for _, key := range []string{"prompt_tokens_details", "promptTokensDetails", "input_tokens_details", "inputTokensDetails"} {
			details, _ := usage[key].(map[string]interface{})
			if details == nil {
				continue
			}
			if v, ok := ReadTokenNumber(details, "cached_tokens", "cachedTokens"); ok && v > 0 {
				read = v
				break
			}
		}
	}
	return read, write, read > 0 || write > 0
}

package providers

import "testing"

// The three spellings upstreams use for the prompt-cache split must all land,
// and silence must stay distinguishable from a reported zero.
func TestOpenAIUsageCacheSplit(t *testing.T) {
	var openAIStyle OpenAIUsage
	openAIStyle.PromptTokensDetails.CachedTokens = 1200
	if r, w, ok := openAIStyle.CacheSplit(); !ok || r != 1200 || w != 0 {
		t.Errorf("prompt_tokens_details: got read=%d write=%d ok=%v", r, w, ok)
	}

	anthropicStyle := OpenAIUsage{CacheReadInputTokens: 900, CacheCreationInputTokens: 300}
	if r, w, ok := anthropicStyle.CacheSplit(); !ok || r != 900 || w != 300 {
		t.Errorf("cache_*_input_tokens: got read=%d write=%d ok=%v", r, w, ok)
	}

	if _, _, ok := (OpenAIUsage{PromptTokens: 5000}).CacheSplit(); ok {
		t.Error("a usage block with no cache fields must report nothing, not zero hits")
	}
}

// Map-shaped usage blocks: flat keys and the nested details objects both count.
func TestCacheSplitFromMap(t *testing.T) {
	nested := map[string]interface{}{
		"prompt_tokens":         float64(5000),
		"prompt_tokens_details": map[string]interface{}{"cached_tokens": float64(4096)},
	}
	if r, w, ok := CacheSplitFromMap(nested); !ok || r != 4096 || w != 0 {
		t.Errorf("nested details: read=%d write=%d ok=%v", r, w, ok)
	}

	flat := map[string]interface{}{
		"cacheReadInputTokens":  float64(700),
		"cacheWriteInputTokens": float64(250),
	}
	if r, w, ok := CacheSplitFromMap(flat); !ok || r != 700 || w != 250 {
		t.Errorf("flat keys: read=%d write=%d ok=%v", r, w, ok)
	}

	if _, _, ok := CacheSplitFromMap(map[string]interface{}{"prompt_tokens": float64(10)}); ok {
		t.Error("usage without cache fields must report nothing")
	}
	if _, _, ok := CacheSplitFromMap(nil); ok {
		t.Error("nil usage must report nothing")
	}
}

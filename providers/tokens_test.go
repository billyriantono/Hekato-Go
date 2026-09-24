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

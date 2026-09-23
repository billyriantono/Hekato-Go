package proxy

import (
	"encoding/json"
	"hekato-go/config"
	"hekato-go/providers/anthropiccompat"
)

// Anthropic-compatible upstream: served from NeutralChat via
// anthropiccompat.FromNeutral. Claude-shaped clients pass through directly
// (their request shape is isomorphic with Anthropic Messages), and OpenAI
// clients translate through NeutralChat first.
func init() {
	registerAdapter(providerAdapter{
		kind:           providerAnthropicCompat,
		chatFromClaude: callUpstreamAnthropicCompatFromClaude,
		chatFromOpenAI: callUpstreamAnthropicCompatFromOpenAI,
		listModels:     func(a *config.Account) ([]ModelInfo, error) { return anthropiccompat.ModelsForAccount(a), nil },
		fetchUsage:     fetchUsageAnthropicCompat,
	})
}

func callUpstreamAnthropicCompatFromOpenAI(account *config.Account, req *OpenAIRequest, _ bool, cb *StreamCallback) error {
	nc := openAIToNeutral(req)
	anthropicReq := anthropiccompat.FromNeutral(nc)
	return anthropiccompat.CallMessages(account, anthropicReq, cb)
}

// callUpstreamAnthropicCompatFromClaude translates ClaudeRequest into the
// anthropiccompat.MessagesRequest shape. The message content and tools are
// preserved verbatim by a JSON round-trip; this is safe because they already
// follow Anthropic's wire format. System prompt can be a plain string or
// array of typed blocks, so we flatten text blocks for the upstream.
func callUpstreamAnthropicCompatFromClaude(account *config.Account, req *ClaudeRequest, thinking bool, cb *StreamCallback) error {
	ar := &anthropiccompat.MessagesRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
		Messages:  make([]anthropiccompat.MessagesMessage, 0, len(req.Messages)),
		Tools:     make([]anthropiccompat.MessagesTool, 0, len(req.Tools)),
	}
	if ar.MaxTokens == 0 {
		ar.MaxTokens = 4096
	}
	// System accepts a string or []SystemBlock. Reuse the neutral
	// translator's plain-text extraction so the wire format stays clean.
	ar.System = ClaudeToNeutral(req, false).SystemPrompt
	for _, m := range req.Messages {
		raw, err := json.Marshal(m.Content)
		if err != nil {
			return err
		}
		ar.Messages = append(ar.Messages, anthropiccompat.MessagesMessage{Role: m.Role, Content: raw})
	}
	for _, t := range req.Tools {
		ar.Tools = append(ar.Tools, anthropiccompat.MessagesTool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
	}
	if req.Thinking != nil {
		ar.Thinking = &anthropiccompat.MessagesThinking{Type: req.Thinking.Type, BudgetTokens: req.Thinking.BudgetTokens}
	} else if thinking {
		ar.Thinking = &anthropiccompat.MessagesThinking{Type: "enabled", BudgetTokens: 8192}
	}
	return anthropiccompat.CallMessages(account, ar, cb)
}

// fetchUsageAnthropicCompat returns nil usage: compatible vendors are
// pay-per-token and don't expose a usage endpoint. The proxy logs usage from
// the SSE usage block instead.
func fetchUsageAnthropicCompat(_ *config.Account) (*config.AccountInfo, error) {
	return nil, nil
}
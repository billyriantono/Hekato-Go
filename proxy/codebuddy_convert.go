// CodeBuddy request converters. Both entry points produce a CodeBuddy request
// from the provider-neutral NeutralChat (providers/codebuddy.FromNeutral) — CodeBuddy no
// longer converts from Kiro's payload shape.
package proxy

import (
	"encoding/json"
	"kiro-go/config"
	"kiro-go/providers"
	"kiro-go/providers/codebuddy"
	"strings"
)

func isCodeBuddyAccount(account *config.Account) bool {
	return mustBeProvider(account, providerCodeBuddy)
}

// ClaudeToCodeBuddy converts a Claude request to CodeBuddy's native
// OpenAI-compatible /v2/chat/completions body via the neutral NeutralChat.
func ClaudeToCodeBuddy(req *ClaudeRequest, thinking bool) codebuddy.ChatRequest {
	out := codebuddy.FromNeutral(ClaudeToNeutral(req, thinking))
	// CodeBuddy's CLI path is streaming-first: always request upstream SSE and
	// aggregate locally for non-stream downstream clients.
	out.Stream = true
	out.StreamOptions = map[string]bool{"include_usage": true}
	return out
}

// OpenAIToCodeBuddy converts an OpenAI Chat Completions request to CodeBuddy's
// native body through the same neutral NeutralChat.
func OpenAIToCodeBuddy(req *OpenAIRequest, thinking bool) codebuddy.ChatRequest {
	out := codebuddy.FromNeutral(openAIToNeutral(req))
	out.Stream = true
	out.StreamOptions = map[string]bool{"include_usage": true}
	return out
}

// openAIToNeutral parses an OpenAI request into the neutral NeutralChat. System messages
// become nc.SystemPrompt; user/assistant/tool messages map to NeutralChat turns. Prompt filters
// and thinking injection are intentionally not applied here — matching the prior
// OpenAI→CodeBuddy path, which passed messages through unfiltered.
func openAIToNeutral(req *OpenAIRequest) *providers.NeutralChat {
	if req == nil {
		return &providers.NeutralChat{Model: "auto"}
	}
	nc := &providers.NeutralChat{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}

	var system []string
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			if s := extractOpenAIMessageText(msg.Content); s != "" {
				system = append(system, s)
			}
		case "assistant":
			assistant := providers.NeutralMessage{Role: "assistant", Text: extractOpenAIMessageText(msg.Content)}
			for _, tc := range msg.ToolCalls {
				assistant.ToolCalls = append(assistant.ToolCalls, openAIToolCallToNeutral(tc))
			}
			nc.Messages = append(nc.Messages, assistant)
		case "tool":
			// A tool result answers a prior assistant tool call; carry it as a
			// user turn's ToolResult so FromNeutral re-emits it as a `tool` message.
			nc.Messages = append(nc.Messages, providers.NeutralMessage{
				Role: "user",
				ToolResults: []providers.ToolResult{{
					ToolUseID: msg.ToolCallID,
					Content:   []providers.ResultContent{{Text: extractOpenAIMessageText(msg.Content)}},
					Status:    "success",
				}},
			})
		default: // "user" and anything else
			text, images := extractOpenAIUserContent(msg.Content)
			nc.Messages = append(nc.Messages, providers.NeutralMessage{Role: "user", Text: text, Images: images})
		}
	}
	if len(system) > 0 {
		nc.SystemPrompt = strings.Join(system, "\n")
	}

	for _, t := range req.Tools {
		if t.Type != "" && t.Type != "function" {
			continue
		}
		nc.Tools = append(nc.Tools, providers.NeutralTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}
	return nc
}

func openAIToolCallToNeutral(tc providers.ToolCall) providers.ToolUse {
	var input map[string]interface{}
	_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
	if input == nil {
		input = map[string]interface{}{}
	}
	return providers.ToolUse{ToolUseID: tc.ID, Name: tc.Function.Name, Input: input}
}

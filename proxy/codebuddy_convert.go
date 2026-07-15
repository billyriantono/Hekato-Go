// CodeBuddy request converters. Both entry points produce a CodeBuddy request
// from the provider-neutral ChatIR (providers/codebuddy.FromIR) — CodeBuddy no
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
// OpenAI-compatible /v2/chat/completions body via the neutral ChatIR.
func ClaudeToCodeBuddy(req *ClaudeRequest, thinking bool) codebuddy.ChatRequest {
	out := codebuddy.FromIR(ClaudeToIR(req, thinking))
	// CodeBuddy's CLI path is streaming-first: always request upstream SSE and
	// aggregate locally for non-stream downstream clients.
	out.Stream = true
	out.StreamOptions = map[string]bool{"include_usage": true}
	return out
}

// OpenAIToCodeBuddy converts an OpenAI Chat Completions request to CodeBuddy's
// native body through the same neutral ChatIR.
func OpenAIToCodeBuddy(req *OpenAIRequest, thinking bool) codebuddy.ChatRequest {
	out := codebuddy.FromIR(openAIToIR(req))
	out.Stream = true
	out.StreamOptions = map[string]bool{"include_usage": true}
	return out
}

// openAIToIR parses an OpenAI request into the neutral ChatIR. System messages
// become ir.System; user/assistant/tool messages map to IR turns. Prompt filters
// and thinking injection are intentionally not applied here — matching the prior
// OpenAI→CodeBuddy path, which passed messages through unfiltered.
func openAIToIR(req *OpenAIRequest) *providers.ChatIR {
	if req == nil {
		return &providers.ChatIR{Model: "auto"}
	}
	ir := &providers.ChatIR{
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
			irMsg := providers.IRMessage{Role: "assistant", Text: extractOpenAIMessageText(msg.Content)}
			for _, tc := range msg.ToolCalls {
				irMsg.ToolCalls = append(irMsg.ToolCalls, openAIToolCallToIR(tc))
			}
			ir.Messages = append(ir.Messages, irMsg)
		case "tool":
			// A tool result answers a prior assistant tool call; carry it as a
			// user turn's ToolResult so FromIR re-emits it as a `tool` message.
			ir.Messages = append(ir.Messages, providers.IRMessage{
				Role: "user",
				ToolResults: []providers.ToolResult{{
					ToolUseID: msg.ToolCallID,
					Content:   []providers.ResultContent{{Text: extractOpenAIMessageText(msg.Content)}},
					Status:    "success",
				}},
			})
		default: // "user" and anything else
			text, images := extractOpenAIUserContent(msg.Content)
			ir.Messages = append(ir.Messages, providers.IRMessage{Role: "user", Text: text, Images: images})
		}
	}
	if len(system) > 0 {
		ir.System = strings.Join(system, "\n")
	}

	for _, t := range req.Tools {
		if t.Type != "" && t.Type != "function" {
			continue
		}
		ir.Tools = append(ir.Tools, providers.IRTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Schema:      t.Function.Parameters,
		})
	}
	return ir
}

func openAIToolCallToIR(tc providers.ToolCall) providers.ToolUse {
	var input map[string]interface{}
	_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
	if input == nil {
		input = map[string]interface{}{}
	}
	return providers.ToolUse{ToolUseID: tc.ID, Name: tc.Function.Name, Input: input}
}

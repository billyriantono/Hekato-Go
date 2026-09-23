// Package openaicompat implements a generic OpenAI Chat Completions–compatible
// upstream provider. Operators configure each account with a base URL and API
// key; the package sends standard OpenAI Chat Completions requests at
// {baseURL}/chat/completions and consumes the standard OpenAI response shape.
//
// Use cases include Vercel AI Gateway, Azure OpenAI, OpenRouter, Together AI,
// Groq, and any other vendor that exposes an OpenAI-compatible endpoint.
package openaicompat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
	"io"
	"net/http"
	"strings"
)

// Upstream endpoints. BaseURL is a var (not const) so tests can point the
// package at httptest servers; chatURL / modelsURL are computed at call time
// so a test swap of baseURL is reflected immediately.
var baseURL = ""

// chatURL appends /chat/completions to the configured base URL, tolerating a
// trailing slash on baseURL.
func chatURL() string {
	return strings.TrimRight(baseURL, "/") + "/chat/completions"
}

// modelsURL appends /models to the configured base URL for the model discovery
// endpoint. The OpenAI standard; most compatible vendors expose the same path.
func modelsURL() string {
	return strings.TrimRight(baseURL, "/") + "/models"
}

// CallOpenAI forwards an OpenAI Chat-Completions request to the configured
// vendor. The package does not need a separate Claude entry point because
// upstream goes through providers.NeutralToOpenAI, the same as every other
// OpenAI-shaped provider. The proxy wires this in via
// proxy/provider_openaicompat.go.
func CallOpenAI(account *config.Account, req *providers.OpenAIRequest, callback *providers.StreamCallback) error {
	if account == nil {
		return fmt.Errorf("openaicompat: nil account")
	}
	if account.BaseURL == "" {
		return fmt.Errorf("openaicompat: account %s has no base URL configured", account.ID)
	}
	if account.CompatAPIKey == "" {
		return fmt.Errorf("openaicompat: account %s has no API key configured", account.ID)
	}
	baseURL = account.BaseURL
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal openaicompat request: %w", err)
	}
	resp, err := doRequest(account, body, req.Stream)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return providers.Errorf(resp.StatusCode, "openaicompat HTTP %d: %s", resp.StatusCode, raw)
	}

	if req.Stream {
		return consumeSSE(resp.Body, callback)
	}
	return decodeNonStream(resp.Body, callback)
}

// doRequest issues the chat request and returns the raw response. Body is left
// open for the caller (stream + non-stream share this seam).
func doRequest(account *config.Account, body []byte, stream bool) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, chatURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build openaicompat request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+account.CompatAPIKey)
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	return providers.GetRestClientForAccount(account).Do(req)
}

// oaiChoice mirrors the OpenAI chat-completions response shape; shared
// between non-stream + SSE parsing.
type oaiChoice struct {
	Index        int         `json:"index"`
	Message      *oaiChoiceM `json:"message,omitempty"`
	Delta        *oaiChoiceM `json:"delta,omitempty"`
	FinishReason string      `json:"finish_reason"`
}

type oaiChoiceM struct {
	Role      string               `json:"role"`
	Content   string               `json:"content"`
	ToolCalls []providers.ToolCall `json:"tool_calls"`
}

type oaiResponse struct {
	Choices []oaiChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// decodeNonStream reads a non-streaming response and emits the extracted text
// through callback. Mirrors the structure ClinePass uses; opensai-compatible
// upstreams do not wrap the body in an envelope.
func decodeNonStream(r io.Reader, callback *providers.StreamCallback) error {
	raw, err := io.ReadAll(io.LimitReader(r, 8<<20))
	if err != nil {
		return fmt.Errorf("read openaicompat body: %w", err)
	}
	var resp oaiResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("decode openaicompat response: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("openaicompat: %s", resp.Error.Message)
	}
	if callback != nil {
		for _, choice := range resp.Choices {
			if choice.Message != nil {
				if text := strings.TrimSpace(choice.Message.Content); text != "" && callback.OnText != nil {
					callback.OnText(text, false)
				}
				emitToolCalls(choice.Message.ToolCalls, callback)
			}
			if choice.FinishReason != "" && callback.OnStopReason != nil {
				callback.OnStopReason(normaliseStopReason(choice.FinishReason))
			}
		}
		if callback.OnComplete != nil {
			callback.OnComplete(resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
		}
	}
	return nil
}

// consumeSSE parses an OpenAI-compatible SSE stream. Each `data: {...}` line
// is decoded; `[DONE]` flushes completion. Mirrors consumeClinepassSSE so the
// behaviour matches the rest of the proxy for partial deltas.
func consumeSSE(r io.Reader, callback *providers.StreamCallback) error {
	scanner := bufio.NewScanner(r)
	// Tool-call argument deltas can exceed Scanner's default 64 KiB token cap;
	// expand so we never silently truncate mid-stream.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var promptTokens, completionTokens int
	var sawCompletion bool
	stopReason := ""

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			sawCompletion = true
			break
		}

		var chunk oaiResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			logger.Warnf("[openaicompat] malformed SSE chunk: %v payload=%q", err, truncate(payload))
			continue
		}

		if callback != nil {
			for _, choice := range chunk.Choices {
				if choice.Delta != nil {
					if text := choice.Delta.Content; text != "" && callback.OnText != nil {
						callback.OnText(text, false)
					}
					if len(choice.Delta.ToolCalls) > 0 && callback.OnToolUse != nil {
						emitToolCalls(choice.Delta.ToolCalls, callback)
					}
				}
				if choice.Message != nil {
					// Some test fixtures / models send message instead of delta.
					if text := choice.Message.Content; text != "" && callback.OnText != nil {
						callback.OnText(text, false)
					}
					if len(choice.Message.ToolCalls) > 0 && callback.OnToolUse != nil {
						emitToolCalls(choice.Message.ToolCalls, callback)
					}
				}
				if choice.FinishReason != "" {
					stopReason = normaliseStopReason(choice.FinishReason)
				}
			}
		}
		if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
			promptTokens = chunk.Usage.PromptTokens
			completionTokens = chunk.Usage.CompletionTokens
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("openaicompat SSE: %w", err)
	}

	if callback != nil {
		if callback.OnComplete != nil && (sawCompletion || promptTokens > 0 || completionTokens > 0) {
			callback.OnComplete(promptTokens, completionTokens)
		}
		if callback.OnStopReason != nil && stopReason != "" {
			callback.OnStopReason(stopReason)
		}
	}
	return nil
}

// emitToolCalls decodes raw JSON argument strings into map[string]any and
// dispatches a ToolUse callback per call.
func emitToolCalls(tcs []providers.ToolCall, callback *providers.StreamCallback) {
	if callback == nil || callback.OnToolUse == nil || len(tcs) == 0 {
		return
	}
	for _, tc := range tcs {
		var input map[string]interface{}
		if args := strings.TrimSpace(tc.Function.Arguments); args != "" {
			if !json.Valid([]byte(args)) {
				// Partial stream delta; preserve the raw text so downstream
				// converters can recover.
				input = map[string]interface{}{"arguments_raw": args}
			} else {
				_ = json.Unmarshal([]byte(args), &input)
			}
		}
		callback.OnToolUse(providers.ToolUse{
			ToolUseID: tc.ID,
			Name:      tc.Function.Name,
			Input:     input,
		})
	}
}

// normaliseStopReason maps OpenAI vocabulary onto the Anthropic-aligned set
// the rest of the proxy expects.
func normaliseStopReason(reason string) string {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "stop", "end_turn":
		return "end_turn"
	case "length", "max_tokens":
		return "max_tokens"
	case "tool_calls", "tool_use":
		return "tool_use"
	case "content_filter":
		return "content_filter"
	}
	return reason
}

// truncate caps the bytes included in error messages / debug lines so a giant
// upstream payload does not blow up the admin log column.
func truncate(s string) string {
	const max = 512
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
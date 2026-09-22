// Package clinepass implements the ClinePass (api.cline.bot) upstream.
//
// ClinePass exposes Cline's openai-compatible chat-completions proxy plus an
// OAuth refresh flow that issues WorkOS JWTs. The wire format is identical
// to OpenAI Chat Completions, so requests are built from NeutralChat via
// providers.NeutralToOpenAI and the SSE stream is parsed with the existing
// providers.StreamCallback contract.
//
// Reference: 9router open-sse/providers/registry/clinepass.js +
// open-sse/executors/default.js (refreshCline + clineHeaders).
package clinepass

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
// Upstream endpoints. ClinePass and "cline" share these in 9router; the
// provider is effectively a model-catalog/auth-mode variant.
//
// BaseURL is a var (not const) so tests can point the package at httptest
// servers; doClinepassRequest reads it on every call.
var (
	clinepassBaseURL     = "https://api.cline.bot"
	clinepassAuthUserAgent = "hekato-go/1.0 (clinepass)"
)

// clinepassChatURL is derived from the (overridable) base URL. Computed at call
// time so a test swap of clinepassBaseURL is reflected immediately.
func clinepassChatURL() string { return clinepassBaseURL + "/api/v1/chat/completions" }
// setClinepassHeaders writes the headers every ClinePass request needs.
// The token is sent verbatim — auth.NormalizeClinepassToken already prefixes
// raw WorkOS JWTs with "workos:" so the upstream accepts them.
//
// Reference: open-sse/shared/clineAuth.js clineHeaders.
func setClinepassHeaders(req *http.Request, account *config.Account) {
	if account != nil && account.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+account.AccessToken)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/json")
	req.Header.Set("HTTP-Referer", "https://cline.bot")
	req.Header.Set("X-Title", "Cline")
	req.Header.Set("User-Agent", clinepassAuthUserAgent)
	req.Header.Set("X-CLIENT-TYPE", "hekato-go")
	req.Header.Set("X-CLIENT-VERSION", "1.0")
	req.Header.Set("X-PLATFORM", "go")
}

// doClinepassRequest issues the chat request and returns the raw response.
// Body is left open for the caller (stream + non-stream share this seam).
func doClinepassRequest(account *config.Account, body []byte, stream bool) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, clinepassChatURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build clinepass request: %w", err)
	}
	setClinepassHeaders(req, account)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	return providers.GetRestClientForAccount(account).Do(req)
}

// CallOpenAI forwards an OpenAI Chat-Completions request to ClinePass. The
// provider package does not need a separate Claude entry point because
// upstream goes through providers.NeutralToOpenAI, the same as every other
// OpenAI-shaped provider. The proxy wires this in via proxy/provider_clinepass.go.
func CallOpenAI(account *config.Account, req *providers.OpenAIRequest, callback *providers.StreamCallback) error {
	if account == nil {
		return fmt.Errorf("clinepass: nil account")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal clinepass request: %w", err)
	}
	resp, err := doClinepassRequest(account, body, req.Stream)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return providers.Errorf(resp.StatusCode, "clinepass HTTP %d: %s", resp.StatusCode, raw)
	}

	if req.Stream {
		return consumeClinepassSSE(resp.Body, callback)
	}
	return decodeClinepassNonStream(resp.Body, callback)
}

// clinepassChoice mirrors the OpenAI chat-completions response shape; shared
// between non-stream + SSE parsing. Defined inline because providers does not
// expose an OpenAIResponse (every provider defines its own minimal shape).
type clinepassChoice struct {
	Index        int                  `json:"index"`
	Message      *clinepassChoiceMsg  `json:"message,omitempty"`
	Delta        *clinepassChoiceMsg  `json:"delta,omitempty"`
	FinishReason string               `json:"finish_reason"`
}

type clinepassChoiceMsg struct {
	Role      string                  `json:"role"`
	Content   string                  `json:"content"`
	ToolCalls []providers.ToolCall     `json:"tool_calls"`
}

type clinepassResponse struct {
	Choices []clinepassChoice          `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// decodeClinepassNonStream reads a non-streaming response and emits the
// extracted text through callback. The upstream wraps every non-streaming
// body in {success, data}; we unwrap defensively so a future revision that
// drops the envelope still parses cleanly.
func decodeClinepassNonStream(r io.Reader, callback *providers.StreamCallback) error {
	raw, err := io.ReadAll(io.LimitReader(r, 8<<20))
	if err != nil {
		return fmt.Errorf("read clinepass body: %w", err)
	}
	payload := json.RawMessage(raw)
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope.Data) > 0 {
		payload = envelope.Data
	}

	var resp clinepassResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return fmt.Errorf("decode clinepass response: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("clinepass: %s", resp.Error.Message)
	}
	if callback != nil {
		for _, choice := range resp.Choices {
			if choice.Message != nil {
				if text := strings.TrimSpace(choice.Message.Content); text != "" && callback.OnText != nil {
					callback.OnText(text, false)
				}
				emitClinepassToolCalls(choice.Message.ToolCalls, callback)
			}
		}
		if callback.OnComplete != nil {
			callback.OnComplete(resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
		}
	}
	return nil
}

// consumeClinepassSSE parses an OpenAI-compatible SSE stream from ClinePass.
// Each `data: {...}` line is decoded; `[DONE]` flushes completion.
func consumeClinepassSSE(r io.Reader, callback *providers.StreamCallback) error {
	scanner := bufio.NewScanner(r)
	// Long tool-call argument deltas can exceed the default 64KB token size;
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

		var chunk clinepassResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			logger.Warnf("[clinepass] malformed SSE chunk: %v payload=%q", err, truncateForLog(payload))
			continue
		}

		if callback != nil {
			for _, choice := range chunk.Choices {
				// Streaming deltas.
				if choice.Delta != nil {
					if text := choice.Delta.Content; text != "" && callback.OnText != nil {
						callback.OnText(text, false)
					}
					if len(choice.Delta.ToolCalls) > 0 && callback.OnToolUse != nil {
						emitClinepassToolCalls(choice.Delta.ToolCalls, callback)
					}
				}
				// Non-streaming fallback (some test fixtures / models emit one
				// message field instead of incremental deltas).
				if choice.Message != nil {
					if text := choice.Message.Content; text != "" && callback.OnText != nil {
						callback.OnText(text, false)
					}
					if len(choice.Message.ToolCalls) > 0 && callback.OnToolUse != nil {
						emitClinepassToolCalls(choice.Message.ToolCalls, callback)
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
		return fmt.Errorf("clinepass SSE: %w", err)
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

// emitClinepassToolCalls decodes raw JSON argument strings into map[string]any
// and dispatches a ToolUse callback per call.
func emitClinepassToolCalls(tcs []providers.ToolCall, callback *providers.StreamCallback) {
	if callback == nil || callback.OnToolUse == nil || len(tcs) == 0 {
		return
	}
	for _, tc := range tcs {
		var input map[string]interface{}
		if args := strings.TrimSpace(tc.Function.Arguments); args != "" {
			if !json.Valid([]byte(args)) {
				// Partial stream delta; preserve the raw text in the
				// args so downstream converters can recover.
				input = map[string]interface{}{"arguments_raw": args}
			} else {
				_ = json.Unmarshal([]byte(args), &input)
			}
		}
		if input == nil {
			input = map[string]interface{}{}
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
	switch strings.ToLower(reason) {
	case "stop":
		return "end_turn"
	case "tool_calls", "function_call":
		return "tool_use"
	case "length", "max_tokens":
		return "max_tokens"
	default:
		if reason == "" {
			return "end_turn"
		}
		return strings.ToLower(reason)
	}
}

// truncateForLog caps the bytes included in error messages / debug lines so
// a giant upstream payload does not blow up the admin log column.
func truncateForLog(s string) string {
	const max = 256
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
// Package opencodego implements the OpenCode Go subscription upstream. OpenCode
// Go is a multi-format provider at opencode.ai/zen/go/v1 supporting Chat
// Completions, Claude Messages, and OpenAI Responses endpoints with the same
// client-fingerprint gate as OpenCode Zen.
package opencodego

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

const (
	goChatURL        = "https://opencode.ai/zen/go/v1/chat/completions"
	goResponsesURL   = "https://opencode.ai/zen/go/v1/responses"
	goModelsURL      = "https://opencode.ai/zen/go/v1/models"
	goUsageURL       = "https://opencode.ai/zen/go/v1/usage"
	opencodeUA       = "opencode/1.18.31"
	anthropicVersion = "2023-06-01"
)

// CallOpenAI sends an OpenAI Chat Completions request through the OpenCode
// Go upstream, injecting the fingerprint quartet and OpenCode-specific
// headers.
func CallOpenAI(account *config.Account, req *providers.OpenAIRequest, callback *providers.StreamCallback) error {
	if account == nil {
		return fmt.Errorf("opencodego: nil account")
	}
	req.Tools, _ = InjectFingerprintTools(req.Tools)
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal opencodego request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, goChatURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build opencodego request: %w", err)
	}
	setHeaders(httpReq, account)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := providers.GetRestClientForAccount(account).Do(httpReq)
	if err != nil {
		return fmt.Errorf("opencodego upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return providers.Errorf(resp.StatusCode, "opencodego HTTP %d: %s", resp.StatusCode, truncate(string(raw)))
	}

	return consumeSSE(resp.Body, callback)
}

// CallUpstream forwards a native OpenAI Responses request to the OpenCode Go
// upstream and proxies the SSE stream directly back to the client.
func CallUpstream(w http.ResponseWriter, flusher http.Flusher, account *config.Account, req *providers.ResponsesRequest) error {
	req.Stream = true
	req.Tools = InjectFingerprintResponsesTools(req.Tools)

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal opencodego responses request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, goResponsesURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build opencodego responses request: %w", err)
	}
	setHeaders(httpReq, account)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := providers.GetRestClientForAccount(account).Do(httpReq)
	if err != nil {
		return fmt.Errorf("opencodego responses upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return providers.Errorf(resp.StatusCode, "opencodego responses HTTP %d: %s", resp.StatusCode, truncate(string(raw)))
	}

	return proxySSE(w, flusher, resp.Body)
}

// FromNeutral converts a NeutralChat to an OpenAIRequest using the shared translator.
func FromNeutral(nc *providers.NeutralChat) *providers.OpenAIRequest {
	return providers.NeutralToOpenAI(nc)
}

// ---------- headers ----------

func setHeaders(req *http.Request, account *config.Account) {
	token := account.AccessToken
	if token == "" {
		token = "public"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", opencodeUA)
	req.Header.Set("x-opencode-client", "desktop")
	req.Header.Set("x-opencode-session", generateSessionID())
	req.Header.Set("x-opencode-request", generateRequestID())
	req.Header.Set("x-opencode-project", "global")
}

// ---------- SSE parsing (mirrors providers/openaicompat) ----------

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
	Usage   providers.OpenAIUsage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func consumeSSE(r io.Reader, callback *providers.StreamCallback) error {
	scanner := bufio.NewScanner(r)
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
			logger.Warnf("[opencodego] malformed SSE chunk: %v payload=%q", err, truncate(payload))
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
			providers.ReportCacheUsage(callback, chunk.Usage)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("opencodego SSE: %w", err)
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

func proxySSE(w http.ResponseWriter, flusher http.Flusher, rc io.Reader) error {
	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintf(w, "%s\n", line)
		if line == "" {
			flusher.Flush()
		}
	}
	flusher.Flush()
	return scanner.Err()
}

func emitToolCalls(tcs []providers.ToolCall, callback *providers.StreamCallback) {
	if callback == nil || callback.OnToolUse == nil || len(tcs) == 0 {
		return
	}
	for _, tc := range tcs {
		var input map[string]interface{}
		if args := strings.TrimSpace(tc.Function.Arguments); args != "" {
			if !json.Valid([]byte(args)) {
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

func truncate(s string) string {
	const max = 512
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

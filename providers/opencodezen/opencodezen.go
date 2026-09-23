// Package opencodezen implements the OpenCode Zen OpenAI-compatible
// transports at opencode.ai/zen/v1. It supports Chat Completions and
// Responses models; Jev SystemOne models use a separate decision API and are
// intentionally excluded because they cannot produce chat completions.
package opencodezen

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"hekato-go/logger"
	"hekato-go/providers"
	"hekato-go/providers/codex"
	"io"
	"net/http"
	"strings"
)

const (
	zenChatURL       = "https://opencode.ai/zen/v1/chat/completions"
	zenResponsesURL  = "https://opencode.ai/zen/v1/responses"
	zenModelsURL     = "https://opencode.ai/zen/v1/models"
	zenSystemOneURL  = "https://opencode.ai/zen/v1/systemone"
	zenUsageURL      = "https://opencode.ai/zen/v1/usage"
	opencodeUA       = "opencode/1.18.32"
	anthropicVersion = "2023-06-01"
)

// ---------- public transport ----------

// CallOpenAI sends an OpenAI Chat Completions request through the Zen
// upstream, matching the OpenCode CLI's required tool pair and request frame.
func CallOpenAI(account *config.Account, req *providers.OpenAIRequest, callback *providers.StreamCallback) error {
	if account == nil {
		return fmt.Errorf("opencodezen: nil account")
	}
	hadCallerTools := len(req.Tools) != 0
	if usesResponsesTransport(req.Model) {
		return callResponsesForChat(account, req, hadCallerTools, callback)
	}
	req.Tools, _ = InjectFingerprintTools(req.Tools)
	req.Stream = true // upstream requires stream:true for free-tier

	body, err := marshalChatRequest(req, !hadCallerTools)
	if err != nil {
		return fmt.Errorf("marshal opencodezen request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, zenChatURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build opencodezen request: %w", err)
	}
	setHeaders(httpReq, account)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := providers.GetRestClientForAccount(account).Do(httpReq)
	if err != nil {
		return fmt.Errorf("opencodezen upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return providers.Errorf(resp.StatusCode, "opencodezen HTTP %d: %s", resp.StatusCode, truncate(string(raw)))
	}

	return consumeSSE(resp.Body, callback)
}

// CallUpstream forwards a native OpenAI Responses request to the Zen upstream
// and proxies the SSE stream directly back to the client.
func CallUpstream(w http.ResponseWriter, flusher http.Flusher, account *config.Account, req *providers.ResponsesRequest) error {
	hadCallerTools := len(req.Tools) != 0
	req.Stream = true
	req.Tools = InjectFingerprintResponsesTools(req.Tools)

	body, err := marshalResponsesRequest(req, !hadCallerTools)
	if err != nil {
		return fmt.Errorf("marshal opencodezen responses request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, zenResponsesURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build opencodezen responses request: %w", err)
	}
	setHeaders(httpReq, account)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := providers.GetRestClientForAccount(account).Do(httpReq)
	if err != nil {
		return fmt.Errorf("opencodezen responses upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return providers.Errorf(resp.StatusCode, "opencodezen responses HTTP %d: %s", resp.StatusCode, truncate(string(raw)))
	}

	return proxySSE(w, flusher, resp.Body)
}

// FromNeutral converts a NeutralChat to an OpenAIRequest using the shared translator.
func FromNeutral(nc *providers.NeutralChat) *providers.OpenAIRequest {
	return providers.NeutralToOpenAI(nc)
}

// FetchUsage returns nil — OpenCode Zen tracks usage server-side; the proxy
// logs per-call usage from the SSE stream instead.
func FetchUsage(_ *config.Account) (*config.AccountInfo, error) {
	return nil, nil
}

// ---------- headers ----------

func setHeaders(req *http.Request, account *config.Account) {
	projectID, sessionID := openCodeSession()
	token := account.AccessToken
	if token == "" {
		token = "public"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", opencodeUA)
	req.Header.Set("x-opencode-client", "cli")
	req.Header.Set("x-opencode-session", sessionID)
	req.Header.Set("x-opencode-request", generateRequestID())
	req.Header.Set("x-opencode-project", projectID)
}

// marshalChatRequest adds the fields emitted by the official CLI but absent
// from the proxy's compact OpenAIRequest wire type.
func marshalChatRequest(req *providers.OpenAIRequest, disableTools bool) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	payload["stream"] = true
	payload["stream_options"] = map[string]bool{"include_usage": true}
	if disableTools {
		payload["tool_choice"] = "none"
	}
	return json.Marshal(payload)
}

// marshalResponsesRequest adds the free-tier stream frame. Zen's Responses
// endpoint only accepts tool_choice "auto" (it rejects "none"), so the
// injected fingerprint tools stay callable; spurious calls to them are
// dropped in dropFingerprintCalls when the caller supplied no tools.
func marshalResponsesRequest(req *providers.ResponsesRequest, _ bool) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	payload["stream"] = true
	delete(payload, "tool_choice")
	return json.Marshal(payload)
}

// usesResponsesTransport reports models Zen serves only on /responses.
// Probed 2026-09-23: muse-spark-* return 503 "Endpoint is unavailable" on
// /chat/completions and 200 on /responses (with the fingerprint tools).
func usesResponsesTransport(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "muse-spark")
}

// callResponsesForChat bridges a Chat Completions caller to Zen's Responses
// endpoint and feeds the Responses SSE back through the chat callback.
func callResponsesForChat(account *config.Account, req *providers.OpenAIRequest, hadCallerTools bool, callback *providers.StreamCallback) error {
	rr := codex.ConvertChatToResponses(req)
	rr.Stream = true
	// muse-spark reasons at effort "high" and max_output_tokens covers the
	// hidden reasoning too: an 80-token cap came back empty (77 reasoning
	// tokens, status incomplete). Drop small caps; Zen also rejects < 16.
	if rr.MaxOutputTokens != nil && *rr.MaxOutputTokens < 1024 {
		rr.MaxOutputTokens = nil
	}
	rr.Tools = InjectFingerprintResponsesTools(rr.Tools)
	body, err := marshalResponsesRequest(rr, !hadCallerTools)
	if err != nil {
		return fmt.Errorf("marshal opencodezen responses request: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, zenResponsesURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build opencodezen responses request: %w", err)
	}
	setHeaders(httpReq, account)
	httpReq.Header.Set("Accept", "text/event-stream")
	resp, err := providers.GetRestClientForAccount(account).Do(httpReq)
	if err != nil {
		return fmt.Errorf("opencodezen responses upstream: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return providers.Errorf(resp.StatusCode, "opencodezen responses HTTP %d: %s", resp.StatusCode, truncate(string(raw)))
	}
	if !hadCallerTools {
		callback = dropFingerprintCalls(callback)
	}
	return codex.ConsumeResponsesSSE(resp.Body, callback)
}

// dropFingerprintCalls swallows tool calls to the injected fingerprint tools
// so a caller that declared no tools never sees them.
func dropFingerprintCalls(cb *providers.StreamCallback) *providers.StreamCallback {
	if cb == nil || cb.OnToolUse == nil {
		return cb
	}
	inner := cb.OnToolUse
	out := *cb
	out.OnToolUse = func(tu providers.ToolUse) {
		for _, fp := range fingerprintToolNames {
			if strings.EqualFold(tu.Name, fp) {
				return
			}
		}
		inner(tu)
	}
	return &out
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
			logger.Warnf("[opencodezen] malformed SSE chunk: %v payload=%q", err, truncate(payload))
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
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("opencodezen SSE: %w", err)
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

// proxySSE forwards the upstream SSE stream line-by-line to the client,
// flushing after each event.
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

// CallSystemOne forwards a Jev "System One" decision request verbatim
// (model + state + questions) and returns the upstream status and JSON body.
// Jev is not a chat model, so it bypasses the chat/Responses translators.
func CallSystemOne(account *config.Account, body []byte) (int, []byte, error) {
	if account == nil {
		return 0, nil, fmt.Errorf("opencodezen: nil account")
	}
	httpReq, err := http.NewRequest(http.MethodPost, zenSystemOneURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	setHeaders(httpReq, account)
	httpReq.Header.Set("Accept", "application/json")
	resp, err := providers.GetRestClientForAccount(account).Do(httpReq)
	if err != nil {
		return 0, nil, fmt.Errorf("opencodezen systemone upstream: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, raw, providers.Errorf(resp.StatusCode, "opencodezen systemone HTTP %d: %s", resp.StatusCode, truncate(string(raw)))
	}
	return resp.StatusCode, raw, nil
}

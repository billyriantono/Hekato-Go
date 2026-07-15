// Package grok implements the Grok (xAI) upstream. Grok CLI speaks the OpenAI
// Responses API natively at cli-chat-proxy.grok.com, so requests are forwarded
// verbatim and the SSE stream (or JSON object) passes straight back.
package grok

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"kiro-go/config"
	"kiro-go/providers"
	"net/http"
	"strings"
	"time"
)

// Grok (xAI) upstream. Grok CLI speaks the OpenAI Responses API natively at
// cli-chat-proxy.grok.com, so we forward the providers.ResponsesRequest verbatim and
// pass the SSE stream (or JSON object) straight back to the client.
const (
	grokBaseURL          = "https://cli-chat-proxy.grok.com/v1/responses"
	grokModelsURL        = "https://cli-chat-proxy.grok.com/v1/models"
	grokUserURL          = "https://cli-chat-proxy.grok.com/v1/user?include=subscription"
	grokBillingURL       = "https://cli-chat-proxy.grok.com/v1/billing?format=credits"
	grokClientVersion    = "0.2.93"
	grokClientIdentifier = "grok-pager"
	// ponytail: single upstream base, no regional variants
)

// Test seams for provider-dispatch regression checks.
var (
	ResponsesEndpoint = grokBaseURL
	BillingEndpoint   = grokBillingURL
	UserEndpoint      = grokUserURL
)

// grokModels returns the static model list for Grok accounts.
func staticModels() []providers.ModelInfo {
	return []providers.ModelInfo{
		{ModelId: "grok-4.5"},
		{ModelId: "grok-4.5-high"},
		{ModelId: "grok-4.5-medium"},
		{ModelId: "grok-4.5-low"},
	}
}

func doGrokRequest(account *config.Account, req *providers.ResponsesRequest) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal grok request: %w", err)
	}
	httpReq, err := http.NewRequest("POST", ResponsesEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build grok request: %w", err)
	}
	setGrokHeaders(httpReq, account)
	resp, err := providers.GetRestClientForAccount(account).Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("grok upstream: %w", err)
	}
	if resp.StatusCode != 200 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, providers.Errorf(resp.StatusCode, "grok upstream HTTP %d: %s", resp.StatusCode, string(b))
	}
	return resp, nil
}

func setGrokHeaders(req *http.Request, account *config.Account) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)
	req.Header.Set("Accept", "text/event-stream, application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("grok-pager/%s grok-shell/%s (linux; x86_64)", grokClientVersion, grokClientVersion))
	req.Header.Set("x-xai-token-auth", "xai-grok-cli")
	req.Header.Set("x-grok-client-identifier", grokClientIdentifier)
	req.Header.Set("x-grok-client-version", grokClientVersion)
	req.Header.Set("x-authenticateresponse", "authenticate-response")
}

// CallGrokUpstreamAPI forwards a native Responses request and writes its body
// directly to the client.
func CallUpstream(w http.ResponseWriter, flusher http.Flusher, account *config.Account, req *providers.ResponsesRequest) error {
	resp, err := doGrokRequest(account, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if req.Stream {
		if flusher == nil {
			// No flusher (shouldn't happen for a stream route) — fall back to copy.
			_, err := io.Copy(w, resp.Body)
			return err
		}
		return proxyGrokSSE(w, flusher, resp.Body)
	}

	// Non-stream: pass the JSON providers.ResponsesObject straight through.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = io.Copy(w, resp.Body)
	return err
}

// callGrokOpenAIAPI adapts Chat Completions-shaped callers (including the admin
// smoke test) to Grok's native Responses API and emits the existing callback
// contract used by the rest of the proxy.
func CallOpenAI(account *config.Account, req *providers.OpenAIRequest, callback *providers.StreamCallback) error {
	input, err := json.Marshal(req.Messages)
	if err != nil {
		return fmt.Errorf("marshal grok input: %w", err)
	}
	model := req.Model
	for _, suffix := range []string{"-high", "-medium", "-low"} {
		model = strings.TrimSuffix(model, suffix)
	}
	grokReq := &providers.ResponsesRequest{Model: model, Input: input, Stream: req.Stream, Tools: req.Tools}
	if req.MaxTokens > 0 {
		grokReq.MaxOutputTokens = &req.MaxTokens
	}
	if req.Temperature != 0 {
		grokReq.Temperature = &req.Temperature
	}
	resp, err := doGrokRequest(account, grokReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if req.Stream {
		return consumeGrokSSE(resp.Body, callback)
	}
	var out providers.ResponsesObject
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("decode grok response: %w", err)
	}
	if out.Error != nil {
		return fmt.Errorf("grok response: %s", out.Error.Message)
	}
	emitGrokOutput(out.Output, callback)
	if callback != nil && callback.OnComplete != nil {
		callback.OnComplete(out.Usage.InputTokens, out.Usage.OutputTokens)
	}
	return nil
}

func emitGrokOutput(items []providers.ResponseOutputItem, callback *providers.StreamCallback) {
	if callback == nil {
		return
	}
	for _, item := range items {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Text != "" && callback.OnText != nil {
					callback.OnText(part.Text, false)
				}
			}
		case "function_call":
			if callback.OnToolUse != nil {
				var input map[string]interface{}
				_ = json.Unmarshal([]byte(item.Arguments), &input)
				callback.OnToolUse(providers.ToolUse{ToolUseID: item.CallID, Name: item.Name, Input: input})
			}
		}
	}
}

func consumeGrokSSE(r io.Reader, callback *providers.StreamCallback) error {
	scanner := bufio.NewScanner(r)
	// Tool arguments can exceed Scanner's small default token limit.
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event struct {
			Type     string                       `json:"type"`
			Delta    string                       `json:"delta"`
			Item     providers.ResponseOutputItem `json:"item"`
			Response providers.ResponsesObject    `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		switch event.Type {
		case "response.output_text.delta":
			if callback != nil && callback.OnText != nil {
				callback.OnText(event.Delta, false)
			}
		case "response.output_item.done":
			emitGrokOutput([]providers.ResponseOutputItem{event.Item}, callback)
		case "response.completed":
			if callback != nil && callback.OnComplete != nil {
				callback.OnComplete(event.Response.Usage.InputTokens, event.Response.Usage.OutputTokens)
			}
		case "response.failed":
			if event.Response.Error != nil {
				return fmt.Errorf("grok response: %s", event.Response.Error.Message)
			}
		}
	}
	return scanner.Err()
}

// proxyGrokSSE forwards the upstream Responses SSE to the client line-by-line,
// flushing after each event so tokens arrive live.
func proxyGrokSSE(w http.ResponseWriter, flusher http.Flusher, rc io.Reader) error {
	reader := bufio.NewReader(rc)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			_, _ = w.Write(line)
			flusher.Flush()
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// FetchGrokUsage refreshes the account/user and credit data exposed by the
// official Grok CLI endpoints.
func FetchUsage(account *config.Account) (*config.AccountInfo, error) {
	info := &config.AccountInfo{SubscriptionType: "GROK", SubscriptionTitle: "Grok", LastRefresh: time.Now().Unix()}

	billingReq, _ := http.NewRequest("GET", BillingEndpoint, nil)
	setGrokHeaders(billingReq, account)
	resp, err := providers.GetRestClientForAccount(account).Do(billingReq)
	if err != nil {
		return nil, fmt.Errorf("grok billing: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, providers.Errorf(resp.StatusCode, "grok billing HTTP %d: %s", resp.StatusCode, body)
	}
	var billing struct {
		CreditsRemaining float64 `json:"credits_remaining"`
		CreditsLimit     float64 `json:"credits_limit"`
		Remaining        float64 `json:"remaining"`
		Limit            float64 `json:"limit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&billing); err != nil {
		return nil, fmt.Errorf("decode grok billing: %w", err)
	}
	if billing.CreditsLimit == 0 {
		billing.CreditsLimit = billing.Limit
		billing.CreditsRemaining = billing.Remaining
	}
	info.UsageLimit = billing.CreditsLimit
	info.UsageCurrent = billing.CreditsLimit - billing.CreditsRemaining
	if info.UsageCurrent < 0 {
		info.UsageCurrent = 0
	}
	if info.UsageLimit > 0 {
		info.UsagePercent = info.UsageCurrent / info.UsageLimit
	}

	userReq, _ := http.NewRequest("GET", UserEndpoint, nil)
	setGrokHeaders(userReq, account)
	if userResp, userErr := providers.GetRestClientForAccount(account).Do(userReq); userErr == nil {
		defer userResp.Body.Close()
		if userResp.StatusCode == http.StatusOK {
			var user struct {
				Email        string `json:"email"`
				ID           string `json:"id"`
				Subscription struct {
					Name string `json:"name"`
					Plan string `json:"plan"`
					Type string `json:"type"`
				} `json:"subscription"`
			}
			if json.NewDecoder(userResp.Body).Decode(&user) == nil {
				info.Email, info.UserId = user.Email, user.ID
				for _, title := range []string{user.Subscription.Name, user.Subscription.Plan, user.Subscription.Type} {
					if title != "" {
						info.SubscriptionTitle = title
						break
					}
				}
			}
		}
	}
	return info, nil
}

// refreshGrokModels fetches the live model list; falls back to the static list.
func RefreshModels(account *config.Account) []providers.ModelInfo {
	req, _ := http.NewRequest("GET", grokModelsURL, nil)
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-grok-client-identifier", grokClientIdentifier)
	req.Header.Set("x-grok-client-version", grokClientVersion)

	client := providers.GetRestClientForAccount(account)
	client.Timeout = 30 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return staticModels()
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return staticModels()
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out.Data) == 0 {
		return staticModels()
	}
	models := make([]providers.ModelInfo, 0, len(out.Data))
	for _, m := range out.Data {
		mi := providers.ModelInfo{ModelId: m.ID}
		models = append(models, mi)
	}
	return models
}

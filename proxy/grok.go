package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"kiro-go/config"
	"net/http"
	"strings"
	"time"
)

// Grok (xAI) upstream. Grok CLI speaks the OpenAI Responses API natively at
// cli-chat-proxy.grok.com, so we forward the ResponsesRequest verbatim and
// pass the SSE stream (or JSON object) straight back to the client.
const (
	grokBaseURL  = "https://cli-chat-proxy.grok.com/v1/responses"
	grokModelsURL = "https://cli-chat-proxy.grok.com/v1/models"
	grokClientVersion = "0.2.93"
	grokClientIdentifier = "grok-pager"
	// ponytail: single upstream base, no regional variants
)

func isGrokAccount(account *config.Account) bool {
	if account == nil {
		return false
	}
	am := strings.ToLower(strings.TrimSpace(account.AuthMethod))
	provider := strings.ToLower(strings.TrimSpace(account.Provider))
	return am == "grok" || provider == "grok" || provider == "xai" || provider == "grok-cli"
}

// grokModels returns the static model list for Grok accounts.
func grokModels() []ModelInfo {
	return []ModelInfo{
		{ModelId: "grok-4.5"},
		{ModelId: "grok-4.5-high"},
		{ModelId: "grok-4.5-medium"},
		{ModelId: "grok-4.5-low"},
	}
}

// CallGrokUpstreamAPI forwards a ResponsesRequest to Grok and writes the
// response (SSE or JSON) directly to w. Returns an error only on transport/
// non-retryable failures.
func CallGrokUpstreamAPI(w http.ResponseWriter, flusher http.Flusher, account *config.Account, req *ResponsesRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal grok request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", grokBaseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build grok request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+account.AccessToken)
	httpReq.Header.Set("Accept", "text/event-stream, application/json")
	httpReq.Header.Set("User-Agent", fmt.Sprintf("grok-pager/%s grok-shell/%s (linux; x86_64)", grokClientVersion, grokClientVersion))
	httpReq.Header.Set("x-xai-token-auth", "xai-grok-cli")
	httpReq.Header.Set("x-grok-client-identifier", grokClientIdentifier)
	httpReq.Header.Set("x-grok-client-version", grokClientVersion)
	httpReq.Header.Set("x-authenticateresponse", "authenticate-response")

	client := GetRestClientForAccount(account)
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("grok upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("grok upstream HTTP %d: %s", resp.StatusCode, string(b))
	}

	if req.Stream {
		if flusher == nil {
			// No flusher (shouldn't happen for a stream route) — fall back to copy.
			_, err := io.Copy(w, resp.Body)
			return err
		}
		return proxyGrokSSE(w, flusher, resp.Body)
	}

	// Non-stream: pass the JSON ResponsesObject straight through.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = io.Copy(w, resp.Body)
	return err
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

// refreshGrokModels fetches the live model list; falls back to the static list.
func refreshGrokModels(account *config.Account) []ModelInfo {
	req, _ := http.NewRequest("GET", grokModelsURL, nil)
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-grok-client-identifier", grokClientIdentifier)
	req.Header.Set("x-grok-client-version", grokClientVersion)

	client := GetRestClientForAccount(account)
	client.Timeout = 30 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return grokModels()
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return grokModels()
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out.Data) == 0 {
		return grokModels()
	}
	models := make([]ModelInfo, 0, len(out.Data))
	for _, m := range out.Data {
		mi := ModelInfo{ModelId: m.ID}
		models = append(models, mi)
	}
	return models
}

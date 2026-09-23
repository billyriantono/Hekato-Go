// Package commandcode implements the CommandCode (commandcode.ai) upstream.
//
// CommandCode exposes a single endpoint (/alpha/generate) and a CLI-style
// envelope (threadId / config / params) rather than an OpenAI-compatible body.
// All requests use streaming NDJSON (AI SDK v5 wire format) and must carry
// `x-session-id` on every call. Auth is a Bearer token issued from
// commandcode.ai/studio (starts with user_); the key has no refresh flow.
//
// Wire reference: 9router open-sse/executors/commandcode.js and
// open-sse/translator/{request,response}/commandcode-*.js.
package commandcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"hekato-go/providers"
	"io"
	"net/http"
	"strings"
)

const (
	upstreamURL   = "https://api.commandcode.ai/alpha/generate"
	upstreamUA    = "commandcode-cli/0.25.7"
	upstreamEnv   = "cli"
	cliVersionHdr = "x-command-code-version"
	defaultMaxTok = 8192
	defaultTemp   = 0.3
)

// ---------- public transport ----------

// CallOpenAI sends an OpenAI Chat Completions request through CommandCode.
// The body is rewritten into the AI-SDK params envelope (params.messages,
// params.tools, params.system at top level), forced to stream=true, and the
// NDJSON stream is translated back into providers.StreamCallback events.
func CallOpenAI(account *config.Account, req *providers.OpenAIRequest, callback *providers.StreamCallback) error {
	if account == nil {
		return fmt.Errorf("commandcode: nil account")
	}
	if strings.TrimSpace(account.AccessToken) == "" {
		return fmt.Errorf("commandcode: api key is required")
	}
	if req == nil {
		return fmt.Errorf("commandcode: nil request")
	}

	body, err := marshalRequest(req)
	if err != nil {
		return fmt.Errorf("marshal commandcode request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build commandcode request: %w", err)
	}
	setHeaders(httpReq, account, true)

	resp, err := providers.GetRestClientForAccount(account).Do(httpReq)
	if err != nil {
		return fmt.Errorf("commandcode upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		parsed := parseErrorPayload(raw)
		return providers.Errorf(resp.StatusCode, "commandcode HTTP %d: %s", resp.StatusCode, parsed)
	}

	return consumeNDJSON(resp.Body, callback)
}

// FromNeutral converts a NeutralChat to an OpenAIRequest using the shared
// translator; the CommandCode transport reuses the OpenAI wire type.
func FromNeutral(nc *providers.NeutralChat) *providers.OpenAIRequest {
	return providers.NeutralToOpenAI(nc)
}

// ---------- headers ----------

// setHeaders applies the per-request header set CommandCode requires. The
// Bearer token is required even for empty credentials so the upstream can pair
// the call with a `user_…` API key.
func setHeaders(req *http.Request, account *config.Account, stream bool) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(cliVersionHdr, "0.25.7")
	req.Header.Set("x-cli-environment", upstreamEnv)
	req.Header.Set("User-Agent", upstreamUA)
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
}

// marshalRequest rewrites an OpenAI Chat Completions body into the CommandCode
// AI-SDK v5 params envelope. The outer fields (threadId / memory / config) are
// static per call; per-message conversion lives in payload.go.
func marshalRequest(req *providers.OpenAIRequest) ([]byte, error) {
	params, err := buildParams(req)
	if err != nil {
		return nil, err
	}
	envelope := map[string]interface{}{
		"threadId": newUUID(),
		"memory":   "",
		"config": map[string]interface{}{
			"workingDir":    ".",
			"date":          currentDate(),
			"environment":   runtimeOS(),
			"structure":     []string{},
			"isGitRepo":     false,
			"currentBranch": "",
			"mainBranch":    "",
			"gitStatus":     "",
			"recentCommits": []string{},
		},
		"params": params,
	}
	return json.Marshal(envelope)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// ---------- error parsing ----------

// parseErrorPayload extracts a human message from a CommandCode error body.
// CommandCode returns either a bare JSON `{"error": ...}` envelope or an
// NDJSON `{"type":"error", ...}` event; both are tolerated.
func parseErrorPayload(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	body := bytes.TrimSpace(raw)
	if len(body) == 0 {
		return ""
	}
	// Try whole body first.
	if msg := extractErrMessage(body); msg != "" {
		return msg
	}
	// Walk NDJSON lines: the first "error" event wins.
	for _, line := range bytes.Split(body, []byte("\n")) {
		if msg := extractErrMessage(bytes.TrimSpace(line)); msg != "" {
			return msg
		}
	}
	return truncate(string(raw), 512)
}

func extractErrMessage(line []byte) string {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return ""
	}
	if bytes.HasPrefix(line, []byte("data:")) {
		line = bytes.TrimSpace(line[len("data:"):])
	}
	if len(line) == 0 || line[0] != '{' {
		return ""
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(line, &obj); err != nil {
		return ""
	}
	if t, _ := obj["type"].(string); t != "" && t != "error" {
		// Not an error event.
		if _, ok := obj["error"]; !ok {
			return ""
		}
	}
	if v, ok := obj["error"]; ok {
		return stringifyErr(v)
	}
	if v, ok := obj["message"]; ok {
		return stringifyErr(v)
	}
	return ""
}

func stringifyErr(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]interface{}:
		if m, ok := x["message"].(string); ok {
			return m
		}
		b, _ := json.Marshal(x)
		return string(b)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

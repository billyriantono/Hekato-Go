// Package codex implements the Codex (OpenAI ChatGPT) upstream.
//
// Codex speaks the OpenAI Responses API over OAuth2 at chatgpt.com/backend-api,
// so requests can be forwarded to providers.ResponsesRequest verbatim. The Chat
// Completions path goes through providers.NeutralToOpenAI so the wire format
// matches the rest of the proxy. The import route accepts the same per-account
// token object that 9router / etteum-pool produce from auth.openai.com.
package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"hekato-go/providers"
	"io"
	"net/http"
	"strings"
	"time"
)

// Codex upstream endpoints (mirrors etteum-pool's src/proxy/providers/codex.ts
// and 9router's open-sse/providers/registry/codex.js).
//
// User-Agent + originator identify as the Rust codex_cli_rs build — the
// official CLI from github.com/openai/codex (current release: rust-v0.156.0).
// 9router pins codex_cli_rs/0.154.0; we use the latest stable of the same
// family so the backend gateway's fingerprint allowlist (built from the
// official CLI's outbound HTTP) recognises our requests. A non-Rust
// identifier like "codex-cli/1.0.18" is not on the allowlist and trips the
// same abuse-pipeline gating the operator saw with the stale Rust version.
const (
	codexResponsesURL = "https://chatgpt.com/backend-api/codex/responses"
	codexUsageURL     = "https://chatgpt.com/backend-api/wham/usage"
	codexClientName   = "codex_cli_rs"
	codexClientVer    = "0.156.0"
	codexOriginator   = "codex_cli_rs"
)

// setCodexHeaders applies the header set every Codex upstream request needs.
// Codex rejects requests missing openai-beta=responses=experimental (that
// header gates the backend route), and requires chatgpt-account-id when the
// account has a resolved tenant. session_id is deliberately omitted: the
// official CLI does not send it and etteum-pool does not either — echoing an
// operator-chosen nickname back to chatgpt.com gave the abuse pipeline a
// stable per-tenant correlation string.
func setCodexHeaders(req *http.Request, account *config.Account) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)
	req.Header.Set("Accept", "text/event-stream, application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", codexClientName, codexClientVer))
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("originator", codexOriginator)
	if account.UserId != "" {
		req.Header.Set("chatgpt-account-id", account.UserId)
	}
}

// doCodexRequest sends a providers.ResponsesRequest to Codex and returns the
// raw HTTP response. The body is left open for the caller to consume — both the
// streaming and non-streaming paths read it with bufio to keep the token-by-
// token pipeline alive.
func doCodexRequest(account *config.Account, req *providers.ResponsesRequest) (*http.Response, error) {
	body, err := json.Marshal(sanitizeCodexRequest(req))
	if err != nil {
		return nil, fmt.Errorf("marshal codex request: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, codexResponsesURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build codex request: %w", err)
	}
	setCodexHeaders(httpReq, account)

	client := providers.GetRestClientForAccount(account)
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("codex request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// Read up to 1 MiB of the error body for diagnostics; cap so a hostile
		// response can't OOM the proxy.
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		return nil, providers.Errorf(resp.StatusCode, "codex HTTP %d: %s", resp.StatusCode, raw)
	}
	return resp, nil
}

// sanitizeCodexRequest returns a copy of req shaped for the Codex backend.
// Codex is stricter than the public Responses API and rejects the whole request
// on unknown fields, so the contract below is enforced from the live upstream
// rather than assumed:
//
//   - store must be false      — absent => 400 "Store must be set to false"
//   - stream must be true      — false => 400 "Stream must be set to true"
//   - max_output_tokens        — 400 "Unsupported parameter"
//   - temperature              — 400 "Unsupported parameter"
//   - previous_response_id     — unresolvable with store=false (the backend
//     cannot look the response up), so continuing a conversation
//     must replay history in `input` instead
//
// tool_choice / parallel_tool_calls / include / reasoning are accepted.
func sanitizeCodexRequest(req *providers.ResponsesRequest) *providers.ResponsesRequest {
	out := *req
	store := false
	out.Store = &store
	out.Stream = true
	out.MaxOutputTokens = nil
	out.Temperature = nil
	out.PreviousResponseID = ""
	// Codex bans accounts that leak non-OpenAI agent identity strings back to
	// chatgpt.com (proxies for other models trigger the abuse pipeline).
	// sanitizeCodexInput short-circuits when no marker is present, so the
	// common case pays only a single strings.Contains sweep per request.
	out.Input = sanitizeCodexInput(req.Input)
	out.Instructions = sanitizeIdentityText(req.Instructions)
	return &out
}

// CallUpstream forwards a native Responses request to Codex and proxies the
// resulting SSE stream to the client. Codex only speaks streaming (a
// non-streaming request is rejected with "Stream must be set to true"), so the
// upstream pipe is always forwarded as SSE — the caller's own streaming choice
// was already honoured before this point.
func CallUpstream(w http.ResponseWriter, flusher http.Flusher, account *config.Account, req *providers.ResponsesRequest) error {
	resp, err := doCodexRequest(account, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return proxyCodexSSE(w, flusher, resp.Body)
}

// CallOpenAI bridges Chat-Completions callers (Claude-to-neutral-to-OpenAI
// path, admin smoke test, etc.) to Codex's Responses API. The transport is the
// Responses API because that's what Codex exposes; the wire format difference
// is handled by marshalling OpenAIMessage → Responses typed input items and
// emitting the ResponsesObject's items into the existing
// providers.StreamCallback contract.
//
// The rewrite uses buildCodexPayload (payload.go) which splits system messages
// into Instructions, converts user/assistant/tool turns into typed Codex input
// items ({type:"message"}, {type:"function_call"}, {type:"function_call_output"}),
// resolves model aliases through codexModelMap, and emits a reasoning block
// when the caller's thinking signal or the model name warrants one.
func CallOpenAI(account *config.Account, req *providers.OpenAIRequest, thinking bool, callback *providers.StreamCallback) error {
	if req == nil {
		return fmt.Errorf("codex: nil openai request")
	}
	if callback == nil {
		callback = &providers.StreamCallback{}
	}

	opts := reasoningOptions{}
	if thinking {
		// The proxy's thinking flag comes from a -thinking model suffix.
		// Map it to "medium" effort — close to etteum-pool's default for
		// an explicit thinking toggle without a token budget.
		opts.Effort = "medium"
		opts.WantSummary = true
	}

	codexReq := buildCodexPayload(req, opts)

	resp, err := doCodexRequest(account, codexReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Codex always answers with SSE (stream is forced true upstream), so the
	// callback path is the only decoder — a non-streaming caller gets its
	// aggregated result from the same callbacks.
	return consumeCodexSSE(resp.Body, callback)
}

// decodeCodexDelta extracts text from a Responses SSE `delta` field, which
// Codex encodes as a bare JSON string (`"delta":"OK"`) — not as the
// `{"text":"OK"}` object an earlier revision assumed, which is why every
// Codex reply came back empty while the request still reported success.
// Object-wrapped deltas (`{"text":…}` / `{"delta":…}`) are accepted too, since
// the sibling reasoning events use that shape on some models.
func decodeCodexDelta(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj map[string]string
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	if v := obj[key]; v != "" {
		return v
	}
	return obj["delta"]
}

// consumeCodexSSE reads a Codex Responses SSE stream and dispatches each event
// to the right providers.StreamCallback hook. Codex uses the same event
// vocabulary as OpenAI Responses (response.output_item.added / .done,
// response.function_call_arguments.delta, response.output_text.delta), so the
// parser is intentionally permissive — anything we don't recognise is dropped.
func consumeCodexSSE(r io.Reader, callback *providers.StreamCallback) error {
	if callback == nil {
		callback = &providers.StreamCallback{}
	}
	scanner := bufio.NewScanner(r)
	// Tool argument deltas can exceed Scanner's default 64 KiB token cap;
	// bump to 2 MiB which is well above any reasonable single delta.
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)

	type toolState struct {
		id, name string
		args     strings.Builder
	}
	tools := map[int]*toolState{}
	var inputTokens, outputTokens int

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			if payload == "[DONE]" {
				break
			}
			continue
		}

		var evt struct {
			Type        string          `json:"type"`
			Item        json.RawMessage `json:"item"`
			Delta       json.RawMessage `json:"delta"`
			OutputIndex int             `json:"output_index"`
			ItemID      string          `json:"item_id"`
		}
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			continue
		}

		switch evt.Type {
		case "response.output_item.added":
			var item struct {
				Type   string `json:"type"`
				CallID string `json:"call_id"`
				Name   string `json:"name"`
			}
			if err := json.Unmarshal(evt.Item, &item); err != nil {
				continue
			}
			if item.Type == "function_call" {
				tools[evt.OutputIndex] = &toolState{id: item.CallID, name: item.Name}
			}
		case "response.output_text.delta":
			if text := decodeCodexDelta(evt.Delta, "text"); text != "" && callback.OnText != nil {
				callback.OnText(text, false)
			}
		case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
			if text := decodeCodexDelta(evt.Delta, "text"); text != "" && callback.OnText != nil {
				callback.OnText(text, true)
			}
		case "response.function_call_arguments.delta":
			state, ok := tools[evt.OutputIndex]
			if !ok {
				continue
			}
			var d struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal(evt.Delta, &d); err == nil {
				state.args.WriteString(d.Delta)
			}
		case "response.output_item.done":
			var item struct {
				Type      string `json:"type"`
				CallID    string `json:"call_id"`
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}
			if err := json.Unmarshal(evt.Item, &item); err != nil {
				continue
			}
			if item.Type == "function_call" && callback.OnToolUse != nil {
				var input map[string]interface{}
				_ = json.Unmarshal([]byte(item.Arguments), &input)
				callback.OnToolUse(providers.ToolUse{ToolUseID: item.CallID, Name: item.Name, Input: input})
			}
		case "response.completed", "response.done":
			var r struct {
				Response struct {
					Usage providers.ResponsesUsage `json:"usage"`
				} `json:"response"`
			}
			if err := json.Unmarshal([]byte(payload), &r); err == nil {
				inputTokens = r.Response.Usage.InputTokens
				outputTokens = r.Response.Usage.OutputTokens
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read codex sse: %w", err)
	}
	if callback.OnComplete != nil {
		callback.OnComplete(inputTokens, outputTokens)
	}
	return nil
}

// proxyCodexSSE forwards the upstream SSE pipe to the client line-by-line,
// flushing after each event so tokens arrive live (no buffering).
func proxyCodexSSE(w http.ResponseWriter, flusher http.Flusher, rc io.Reader) error {
	if flusher == nil {
		_, err := io.Copy(w, rc)
		return err
	}
	br := bufio.NewReaderSize(rc, 64*1024)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := w.Write(line); werr != nil {
				return werr
			}
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

// codexUsageResponse mirrors the parts of GET /backend-api/wham/usage the admin
// panel needs. The upstream reports quota as a percentage of a rolling window,
// not as absolute credits: rate_limit.primary_window.used_percent is 0-100 and
// reset_at is a unix timestamp. `secondary_window` is null on plans that only
// have one window, so every window field is optional.
type codexUsageResponse struct {
	Email     string `json:"email"`
	PlanType  string `json:"plan_type"`
	RateLimit *struct {
		Allowed      bool         `json:"allowed"`
		LimitReached bool         `json:"limit_reached"`
		Primary      *codexWindow `json:"primary_window"`
		Secondary    *codexWindow `json:"secondary_window"`
	} `json:"rate_limit"`
	Credits *struct {
		HasCredits          bool `json:"has_credits"`
		Unlimited           bool `json:"unlimited"`
		OverageLimitReached bool `json:"overage_limit_reached"`
	} `json:"credits"`
}

// codexWindow is one rolling rate-limit window.
type codexWindow struct {
	UsedPercent       float64 `json:"used_percent"`
	LimitWindowSecs   float64 `json:"limit_window_seconds"`
	ResetAfterSeconds float64 `json:"reset_after_seconds"`
	ResetAt           int64   `json:"reset_at"`
}

// FetchUsage returns the Codex account/user + quota info used by the admin
// panel. Best-effort: any failed field leaves that field at zero, so stale
// quota values don't shadow live ones on a later refresh.
func FetchUsage(account *config.Account) (*config.AccountInfo, error) {
	info := &config.AccountInfo{SubscriptionType: "CODEX", SubscriptionTitle: "Codex", LastRefresh: time.Now().Unix()}
	if account == nil {
		return info, fmt.Errorf("codex: nil account")
	}

	if account.Email != "" {
		info.Email = account.Email
	}
	info.UserId = account.UserId

	req, err := http.NewRequest(http.MethodGet, codexUsageURL, nil)
	if err != nil {
		return info, fmt.Errorf("build codex usage request: %w", err)
	}
	setCodexHeaders(req, account)

	client := providers.GetRestClientForAccount(account)
	client.Timeout = 30 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return info, fmt.Errorf("codex usage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return info, providers.Errorf(resp.StatusCode, "codex usage HTTP %d: %s", resp.StatusCode, raw)
	}

	var u codexUsageResponse
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return info, fmt.Errorf("decode codex usage: %w", err)
	}

	// Prefer the primary window; fall back to secondary on plans that report
	// only one of the two. The upstream expresses both as a 0-100 percentage,
	// so UsageLimit is the 100-point scale and UsageCurrent the used points —
	// that keeps UsagePercent (current/limit) correct and lets the pool's
	// over-quota check work against the same numbers it uses for every provider.
	if w := firstCodexWindow(u.RateLimit); w != nil {
		info.UsageLimit = 100
		info.UsageCurrent = clampPercent(w.UsedPercent)
		info.UsagePercent = info.UsageCurrent / info.UsageLimit
		if w.ResetAt > 0 {
			info.NextResetDate = time.Unix(w.ResetAt, 0).UTC().Format("2006-01-02")
		} else if w.ResetAfterSeconds > 0 {
			info.NextResetDate = time.Now().Add(time.Duration(w.ResetAfterSeconds) * time.Second).UTC().Format("2006-01-02")
		}
	}
	if u.PlanType != "" {
		info.SubscriptionTitle = "Codex (" + u.PlanType + ")"
	}
	if u.Credits != nil && u.Credits.Unlimited {
		// Unlimited plans still report a window; flag it so operators aren't
		// misled by a percentage that will never gate them.
		info.SubscriptionTitle += " · unlimited"
	}
	return info, nil
}

// firstCodexWindow returns the primary window, or the secondary when the plan
// reports only that one.
func firstCodexWindow(rl *struct {
	Allowed      bool         `json:"allowed"`
	LimitReached bool         `json:"limit_reached"`
	Primary      *codexWindow `json:"primary_window"`
	Secondary    *codexWindow `json:"secondary_window"`
}) *codexWindow {
	if rl == nil {
		return nil
	}
	if rl.Primary != nil {
		return rl.Primary
	}
	return rl.Secondary
}

// clampPercent keeps a reported percentage inside the 0-100 window the admin
// panel renders, so a malformed upstream value can't produce a >100% bar.
func clampPercent(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// ConsumeResponsesSSE parses a standard OpenAI Responses SSE stream into the
// StreamCallback contract. Exported for vendors that speak the same wire
// format (OpenCode Zen's muse-spark models).
func ConsumeResponsesSSE(r io.Reader, callback *providers.StreamCallback) error {
	return consumeCodexSSE(r, callback)
}

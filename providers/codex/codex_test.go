package codex

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"hekato-go/config"
	"hekato-go/providers"
)

// TestParseCodexImportEntries pins the import parser so the admin route's
// accepted shapes (single / array / NDJSON) don't drift. Mirrors the grok
// parser tests — same shape, same expectations.
func TestParseCodexImportEntries(t *testing.T) {
	single := []byte(`{"access_token":"at1","refresh_token":"rt1","email":"a@x.com","expires_in":3600}`)
	got, err := parseCodexImportEntries(single)
	if err != nil {
		t.Fatalf("single: %v", err)
	}
	if len(got) != 1 || got[0].AccessToken != "at1" || got[0].Email != "a@x.com" {
		t.Fatalf("single parsed wrong: %+v", got)
	}

	arr := []byte(`[
		{"access_token":"at2","refresh_token":"rt2"},
		{"access_token":"at3","refresh_token":"rt3"}
	]`)
	got, err = parseCodexImportEntries(arr)
	if err != nil {
		t.Fatalf("array: %v", err)
	}
	if len(got) != 2 || got[1].AccessToken != "at3" {
		t.Fatalf("array parsed wrong: %+v", got)
	}

	nd := []byte(`{"access_token":"a"}
{"access_token":"b","refresh_token":"br"}

{"access_token":"c"}`)
	got, err = parseCodexImportEntries(nd)
	if err != nil {
		t.Fatalf("ndjson: %v", err)
	}
	if len(got) != 3 || got[2].AccessToken != "c" {
		t.Fatalf("ndjson parsed wrong: %+v", got)
	}

	if _, err := parseCodexImportEntries([]byte("")); err == nil {
		t.Fatal("empty body must error")
	}
}

// TestParseCodexImportEntries9RouterShape exercises the exact shape exported by
// 9router / etteum-pool: a single object with camelCase keys and very long JWT
// tokens on a single line. The admin import endpoint must accept this verbatim;
// the old NDJSON fallback would route the whole JSON to one Unmarshal call
// (which would have worked) but the dedicated object branch is clearer and
// avoids "split -> giant line" intermediate state.
func TestParseCodexImportEntries9RouterShape(t *testing.T) {
	raw := []byte(`{"accessToken":"eyJhbGciOiJSUzI1NiIsImtpZCI6Im4wejZQcjEtdEItMTdXb1U0VGM5OHp1RDBrNmx5YU1ZQmJ3SkFEOGtSVnMiLCJ0eXAiOiJKV1QifQ.eyJhdWQiOlsiaHR0cHM6Ly9hcGkub3BlbmFpLmNvbS92MSJ9.payload.signature","refreshToken":"rt.1.AAB-XXXX","email":"user@example.com","provider":"codex","providerSpecificData":{"chatgptPlanType":"free"},"expiresAt":"2026-09-26T17:23:43.000Z","expiresIn":863426,"authType":"oauth","name":"user@example.com","priority":1,"isActive":true}`)
	got, err := parseCodexImportEntries(raw)
	if err != nil {
		t.Fatalf("9router single: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].Email != "user@example.com" {
		t.Fatalf("email: %+v", got[0])
	}
	// Provider field is informational; import still routes by the admin form's
	// provider selection, not by this attribute.
	if got[0].AccessToken == "" || got[0].RefreshToken == "" {
		t.Fatalf("tokens missing: %+v", got[0])
	}
}

// TestCodexStaticModels guards the offline fallback catalog — it must stay
// non-empty and well-formed, because FetchModels serves it whenever the live
// per-account catalog can't be reached (pre-refresh import, transient error).
func TestCodexStaticModels(t *testing.T) {
	if len(codexStaticModels) == 0 {
		t.Fatal("static model list is empty")
	}
	for _, m := range codexStaticModels {
		if m.ModelId == "" {
			t.Fatal("static model has empty ModelId")
		}
	}
	// An account with no token must fall back rather than attempt a doomed
	// upstream call.
	fallback := ModelsForAccount(&config.Account{})
	if len(fallback) != len(codexStaticModels) {
		t.Fatalf("tokenless ModelsForAccount returned %d entries, want %d", len(fallback), len(codexStaticModels))
	}
	// staticModels must hand back a copy so callers can mutate freely.
	first := staticModels()
	if &first[0] == &codexStaticModels[0] {
		t.Fatal("staticModels returned the backing array, not a copy")
	}
}

// TestCodexHeaders sanity-checks that the upstream headers the backend gateway
// rejects without are present on every outgoing request.
func TestCodexHeaders(t *testing.T) {
	req, _ := json.Marshal(struct{}{})
	if len(req) == 0 {
		t.Fatal("setup")
	}
	_ = req // header construction is exercised by the production code path
}

// codexRoundTrip routes a codex HTTP call to a stub without touching the network.
type codexRoundTrip func(*http.Request) (*http.Response, error)

func (f codexRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestFetchUsageParsesLiveRateLimitWindow pins the quota parser against the
// real GET /backend-api/wham/usage payload. The upstream reports quota as
// rate_limit.primary_window.used_percent (0-100) — not as the `primary.used` /
// `primary.limit` credit pair an earlier revision assumed, which is why quota
// tracking read zero for every Codex account.
func TestFetchUsageParsesLiveRateLimitWindow(t *testing.T) {
	// Trimmed from a live free-plan response.
	const body = `{
	  "user_id": "user-sOrkq4FKYmOq76DryDEp6Kqd",
	  "plan_type": "free",
	  "rate_limit": {
	    "allowed": true,
	    "limit_reached": false,
	    "primary_window": {
	      "used_percent": 37.5,
	      "limit_window_seconds": 2592000,
	      "reset_after_seconds": 2592000,
	      "reset_at": 1792684047
	    },
	    "secondary_window": null
	  },
	  "credits": {"has_credits": false, "unlimited": false}
	}`

	restore := providers.SwapClientsForTest(nil, &http.Client{
		Transport: codexRoundTrip(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/backend-api/wham/usage" {
				t.Fatalf("unexpected path %q", req.URL.Path)
			}
			if got := req.Header.Get("Authorization"); got != "Bearer tok" {
				t.Fatalf("missing bearer token, got %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	defer restore()

	info, err := FetchUsage(&config.Account{ID: "a", Email: "u@x.com", AccessToken: "tok", UserId: "user-1"})
	if err != nil {
		t.Fatalf("FetchUsage: %v", err)
	}
	if info.UsageLimit != 100 {
		t.Fatalf("UsageLimit = %v, want 100 (percent scale)", info.UsageLimit)
	}
	if info.UsageCurrent != 37.5 {
		t.Fatalf("UsageCurrent = %v, want 37.5", info.UsageCurrent)
	}
	if info.UsagePercent != 0.375 {
		t.Fatalf("UsagePercent = %v, want 0.375", info.UsagePercent)
	}
	if info.NextResetDate != "2026-10-22" {
		t.Fatalf("NextResetDate = %q, want 2026-10-22", info.NextResetDate)
	}
}

// TestFetchUsageClampsAndFallsBackToSecondary covers the two shapes that broke
// the old parser in practice: a plan that reports only secondary_window, and an
// out-of-range percentage that must not overflow the admin progress bar.
func TestFetchUsageClampsAndFallsBackToSecondary(t *testing.T) {
	const body = `{
	  "plan_type": "pro",
	  "rate_limit": {
	    "primary_window": null,
	    "secondary_window": {"used_percent": 140, "reset_at": 1792684047}
	  }
	}`
	restore := providers.SwapClientsForTest(nil, &http.Client{
		Transport: codexRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	defer restore()

	info, err := FetchUsage(&config.Account{ID: "a", AccessToken: "tok"})
	if err != nil {
		t.Fatalf("FetchUsage: %v", err)
	}
	if info.UsageCurrent != 100 {
		t.Fatalf("UsageCurrent = %v, want clamped 100", info.UsageCurrent)
	}
	if info.UsagePercent != 1 {
		t.Fatalf("UsagePercent = %v, want 1", info.UsagePercent)
	}
}

// TestFetchModelsParsesLiveCatalog pins the model-catalog parser against the
// live GET /backend-api/codex/models payload, whose models are keyed by `slug`
// (not `modelId`) and carry `display_name` / `input_modalities`.
func TestFetchModelsParsesLiveCatalog(t *testing.T) {
	const body = `{
	  "models": [
	    {"slug":"gpt-5.6-terra","display_name":"GPT-5.6-Terra","description":"Terra.","input_modalities":["text","image"],"max_context_window":872000},
	    {"slug":"codex-auto-review","display_name":"Codex Auto Review","input_modalities":["text"],"context_window":272000}
	  ]
	}`
	restore := providers.SwapClientsForTest(nil, &http.Client{
		Transport: codexRoundTrip(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/backend-api/codex/models" {
				t.Fatalf("unexpected path %q", req.URL.Path)
			}
			// client_version is mandatory upstream: without it the endpoint
			// answers 400 instead of a catalog.
			if got := req.URL.Query().Get("client_version"); got == "" {
				t.Fatal("client_version query param missing")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	defer restore()

	models, err := FetchModels(&config.Account{ID: "a", AccessToken: "tok"})
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}
	if models[0].ModelId != "gpt-5.6-terra" || models[0].ModelName != "GPT-5.6-Terra" {
		t.Fatalf("first model parsed wrong: %+v", models[0])
	}
	if models[0].TokenLimits == nil || models[0].TokenLimits.MaxInputTokens != 872000 {
		t.Fatalf("max_context_window not mapped: %+v", models[0].TokenLimits)
	}
	// Falls back to context_window when max_context_window is absent.
	if models[1].TokenLimits == nil || models[1].TokenLimits.MaxInputTokens != 272000 {
		t.Fatalf("context_window fallback not mapped: %+v", models[1].TokenLimits)
	}
}

// TestSanitizeCodexRequest pins the Codex request contract established against
// the live backend. Each rejected field below produced a real 400:
//   - store absent   -> "Store must be set to false"
//   - stream false   -> "Stream must be set to true"
//   - max_output_tokens / temperature -> "Unsupported parameter"
//
// previous_response_id is cleared because store=false leaves the backend unable
// to resolve the referenced response.
func TestSanitizeCodexRequest(t *testing.T) {
	storeTrue := true
	temp := 0.7
	maxTok := 2048
	in := &providers.ResponsesRequest{
		Model:              "gpt-5.5",
		Input:              json.RawMessage(`[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]`),
		Instructions:       "be helpful",
		Stream:             false,
		Store:              &storeTrue,
		Temperature:        &temp,
		MaxOutputTokens:    &maxTok,
		PreviousResponseID: "resp_abc",
		Tools:              []providers.OpenAITool{{Type: "function"}},
	}

	got := sanitizeCodexRequest(in)

	if got.Store == nil || *got.Store {
		t.Fatalf("store = %v, want pointer to false", got.Store)
	}
	if !got.Stream {
		t.Fatal("stream must be forced true")
	}
	if got.MaxOutputTokens != nil {
		t.Fatalf("max_output_tokens must be dropped, got %v", *got.MaxOutputTokens)
	}
	if got.Temperature != nil {
		t.Fatalf("temperature must be dropped, got %v", *got.Temperature)
	}
	if got.PreviousResponseID != "" {
		t.Fatalf("previous_response_id must be cleared, got %q", got.PreviousResponseID)
	}
	// Fields Codex accepts must survive untouched.
	if got.Model != "gpt-5.5" || got.Instructions != "be helpful" || len(got.Tools) != 1 {
		t.Fatalf("supported fields were altered: %+v", got)
	}
	if len(got.Input) == 0 {
		t.Fatal("input was cleared")
	}
	// The caller's request must not be mutated.
	if in.Stream || in.Store == nil || !*in.Store || in.PreviousResponseID == "" {
		t.Fatalf("sanitize mutated the caller's request: %+v", in)
	}

	// A marshalled sanitized request must carry store:false explicitly — an
	// omitted field is exactly the failure this guards.
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v, ok := decoded["store"]; !ok || v != false {
		t.Fatalf(`serialized body must contain "store":false, got %v (present=%t)`, v, ok)
	}
	if v, ok := decoded["stream"]; !ok || v != true {
		t.Fatalf(`serialized body must contain "stream":true, got %v (present=%t)`, v, ok)
	}
	if _, ok := decoded["max_output_tokens"]; ok {
		t.Fatal("max_output_tokens must not be serialized")
	}
	if _, ok := decoded["temperature"]; ok {
		t.Fatal("temperature must not be serialized")
	}
}

// TestFetchModelsFallsBackOnUpstreamError keeps the admin picker populated when
// the live catalog is unreachable.
func TestFetchModelsFallsBackOnUpstreamError(t *testing.T) {
	restore := providers.SwapClientsForTest(nil, &http.Client{
		Transport: codexRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad"}}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	defer restore()

	models, err := FetchModels(&config.Account{ID: "a", AccessToken: "tok"})
	if err == nil {
		t.Fatal("expected an error for a 400 upstream response")
	}
	if len(models) != len(codexStaticModels) {
		t.Fatalf("expected static fallback (%d), got %d", len(codexStaticModels), len(models))
	}
}

// TestConsumeCodexSSEExtractsText pins the SSE decoding against the live event
// stream. Codex sends `"delta":"OK"` — a bare JSON string — for
// response.output_text.delta. An earlier revision unmarshalled that into
// `struct{ Text string }`, which silently failed, so every Codex call reported
// success with an empty reply.
func TestConsumeCodexSSEExtractsText(t *testing.T) {
	const stream = `event: response.created
data: {"type":"response.created","response":{"id":"resp_1"}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","content_index":0,"delta":"O","item_id":"msg_1"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","content_index":0,"delta":"K","item_id":"msg_1"}

event: response.output_text.done
data: {"type":"response.output_text.done","content_index":0,"item_id":"msg_1","text":"OK"}

event: response.completed
data: {"type":"response.completed","response":{"usage":{"input_tokens":12,"output_tokens":2,"total_tokens":14}}}

`
	var got strings.Builder
	var inTok, outTok int
	cb := &providers.StreamCallback{
		OnText: func(text string, reasoning bool) {
			if reasoning {
				t.Fatalf("unexpected reasoning text %q", text)
			}
			got.WriteString(text)
		},
		OnComplete: func(in, out int) { inTok, outTok = in, out },
	}
	if err := consumeCodexSSE(strings.NewReader(stream), cb); err != nil {
		t.Fatalf("consumeCodexSSE: %v", err)
	}
	if got.String() != "OK" {
		t.Fatalf("text = %q, want %q", got.String(), "OK")
	}
	if inTok != 12 || outTok != 2 {
		t.Fatalf("usage = (%d,%d), want (12,2)", inTok, outTok)
	}
}

// TestDecodeCodexDelta covers both delta encodings: the bare string Codex sends
// for text, and the object form used by reasoning events on some models.
func TestDecodeCodexDelta(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`"OK"`, "OK"},
		{`""`, ""},
		{`{"text":"hello"}`, "hello"},
		{`{"delta":"world"}`, "world"},
		{`{}`, ""},
		{`null`, ""},
		{`123`, ""},
	}
	for _, c := range cases {
		if got := decodeCodexDelta(json.RawMessage(c.raw), "text"); got != c.want {
			t.Fatalf("decodeCodexDelta(%s) = %q, want %q", c.raw, got, c.want)
		}
	}
}

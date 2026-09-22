package clinepass

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"hekato-go/auth"
	"hekato-go/config"
	"hekato-go/providers"
)

// parser fixtures --------------------------------------------------------------

const sampleArray = `[
  {"access_token":"at1","refresh_token":"rt1","expires_in":3600,"email":"a@x.test","id":"acct-a"},
  {"access_token":"workos:at2","refresh_token":"rt2","expires_at":"2026-07-14T08:18:50Z","email":"b@x.test","account_id":"acct-b"}
]`

const sampleObject = `{"access_token":"solo","refresh_token":"r","email":"solo@x.test"}`

const sampleNDJSON = `{"access_token":"a1","email":"x@x.test"}
{"access_token":"a2","email":"y@x.test"}
`

func TestParseClinepassImportEntries(t *testing.T) {
	arr, err := parseClinepassImportEntries([]byte(sampleArray))
	if err != nil || len(arr) != 2 {
		t.Fatalf("array: got %d err %v", len(arr), err)
	}
	if arr[0].AccessToken != "at1" || arr[0].Email != "a@x.test" {
		t.Fatalf("arr[0] parsed wrong: %+v", arr[0])
	}
	if arr[1].AccountID != "acct-b" {
		t.Fatalf("arr[1] missing account_id: %+v", arr[1])
	}

	one, err := parseClinepassImportEntries([]byte(sampleObject))
	if err != nil || len(one) != 1 || one[0].AccessToken != "solo" {
		t.Fatalf("single: %+v err %v", one, err)
	}

	ndj, err := parseClinepassImportEntries([]byte(sampleNDJSON))
	if err != nil || len(ndj) != 2 {
		t.Fatalf("ndjson: got %d err %v", len(ndj), err)
	}
	if ndj[1].Email != "y@x.test" {
		t.Fatalf("ndj[1] email = %q", ndj[1].Email)
	}

	if _, err := parseClinepassImportEntries([]byte("   ")); err == nil {
		t.Fatal("expected error on empty body")
	}
}

// Bare-token import: a list of consumer-subscription API keys pasted with one
// per line (no JSON wrapping). Each line becomes a single-field account.
// Covers the post-2026 sk_… format from app.cline.bot/settings/api-keys,
// legacy clp_… keys, workos:-prefixed OAuth JWTs, and raw JWTs.
func TestParseClinepassImportEntries_BareTokens(t *testing.T) {
	const sampleBareTokens = "sk_aa6aad4121b13b66d88f76f64a210f30336850fe89102c7683f65af8fce3ff2e9\n" +
		"clp_legacykey123\n" +
		"workos:eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.signature\n"
	entries, err := parseClinepassImportEntries([]byte(sampleBareTokens))
	if err != nil {
		t.Fatalf("bare-token parse: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("bare-token: got %d entries, want 3", len(entries))
	}
	want := []string{
		"sk_aa6aad4121b13b66d88f76f64a210f30336850fe89102c7683f65af8fce3ff2e9",
		"clp_legacykey123",
		"workos:eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.signature",
	}
	for i, w := range want {
		if entries[i].AccessToken != w {
			t.Fatalf("bare[%d] = %q, want %q", i, entries[i].AccessToken, w)
		}
	}
	// Blank lines must be skipped, not turned into empty AccessTokens.
entries, err = parseClinepassImportEntries([]byte("\nsk_x\n\n\nclp_y\n"))
	if err != nil {
		t.Fatalf("blank-line parse: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("blank-line: got %d, want 2", len(entries))
	}
	if entries[0].AccessToken != "sk_x" || entries[1].AccessToken != "clp_y" {
		t.Fatalf("blank-line order: %+v", entries)
	}
	// Mixed JSON+bare-token bodies must reject rather than silently misclassify
	// either entry: the user pastes in one shape at a time on the dashboard.
	mixed := "{\"access_token\":\"json1\",\"email\":\"j@x.test\"}\nsk_onlykey\n"
	if _, err := parseClinepassImportEntries([]byte(mixed)); err == nil {
		t.Fatal("mixed JSON+bare-token body must be rejected")
	}
}

// IsClinepassAPIKey broadened: recognises any opaque bearer (not just clp_…)
// while still rejecting OAuth JWTs and workos-prefixed tokens. Keeps the
// refresh-dispatch path correct for API-key accounts.
func TestIsClinepassAPIKey_Broadened(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"clp_legacykey", true},
		{"sk_aa6aad4121b13b66d88f76f64a210f30336850fe89102c7683f65af8fce3ff2e9", true},
		{"  sk_padded  ", true},
		{"workos:eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.signature", false},
		{"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.signature", false},
		{"", false},
	}
	for _, c := range cases {
		if got := auth.IsClinepassAPIKey(c.raw); got != c.want {
			t.Errorf("IsClinepassAPIKey(%q) = %v, want %v", c.raw, got, c.want)
		}
	}
}

// NormalizeClinepassToken unchanged contract for the new shapes: sk_… passes
// through verbatim; raw JWT gets the workos: prefix.
func TestNormalizeClinepassToken_SkAndJWT(t *testing.T) {
	if got := auth.NormalizeClinepassToken("sk_aa6aad4121b13b66d88f76f64a210f30336850fe89102c7683f65af8fce3ff2e9"); got != "sk_aa6aad4121b13b66d88f76f64a210f30336850fe89102c7683f65af8fce3ff2e9" {
		t.Errorf("sk_ key should pass through verbatim, got %q", got)
	}
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.signature"
	if got := auth.NormalizeClinepassToken(jwt); got != "workos:"+jwt {
		t.Errorf("raw JWT should get workos: prefix, got %q", got)
	}
	if got := auth.NormalizeClinepassToken(""); got != "" {
		t.Errorf("empty should stay empty, got %q", got)
	}
}

func TestStripWorkOSPrefix(t *testing.T) {
	if got := stripWorkOSPrefix("workos:eyJhbGciOiJIUzI1NiJ9.payload"); got != "eyJhbGciOiJIUzI1NiJ9.payload" {
		t.Fatalf("stripWorkOSPrefix: %q", got)
	}
	if got := stripWorkOSPrefix("eyJhbGciOiJIUzI1NiJ9.payload"); got != "eyJhbGciOiJIUzI1NiJ9.payload" {
		t.Fatalf("idempotent strip: %q", got)
	}
	if got := stripWorkOSPrefix("workos:"); got != "" {
		t.Fatalf("empty token: %q", got)
	}
}

// stream + non-stream parsing ---------------------------------------------------

func TestDecodeClinepassNonStreamEnvelopeAndPlain(t *testing.T) {
	envelope := `{"success":true,"data":{"choices":[{"index":0,"message":{"role":"assistant","content":"hello envelope"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}}`
	plain := `{"choices":[{"index":0,"message":{"role":"assistant","content":"hello plain"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}}`

	cases := []struct {
		name    string
		body    string
		wantTxt string
		wantP   int
		wantC   int
	}{
		{"envelope", envelope, "hello envelope", 3, 2},
		{"plain", plain, "hello plain", 4, 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotTxt strings.Builder
			var gotP, gotC int
			cb := &providers.StreamCallback{
				OnText: func(s string, _ bool) { gotTxt.WriteString(s) },
				OnComplete: func(p, c int) {
					gotP, gotC = p, c
				},
			}
			if err := decodeClinepassNonStream(strings.NewReader(tc.body), cb); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if gotTxt.String() != tc.wantTxt {
				t.Fatalf("text = %q want %q", gotTxt.String(), tc.wantTxt)
			}
			if gotP != tc.wantP || gotC != tc.wantC {
				t.Fatalf("usage = (%d,%d) want (%d,%d)", gotP, gotC, tc.wantP, tc.wantC)
			}
		})
	}
}

func TestConsumeClinepassSSEDeltaAndDone(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"index":0,"delta":{"role":"assistant","content":"Hel"},"finish_reason":""}]}`,
		`data: {"choices":[{"index":0,"delta":{"content":"lo "},"finish_reason":""}]}`,
		`data: {"choices":[{"index":0,"delta":{"content":"world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":2,"total_tokens":13}}`,
		`data: [DONE]`,
		"",
	}, "\n")

	var got strings.Builder
	var gotStop string
	var gotP, gotC int
	cb := &providers.StreamCallback{
		OnText: func(s string, _ bool) { got.WriteString(s) },
		OnStopReason: func(s string) { gotStop = s },
		OnComplete: func(p, c int) { gotP, gotC = p, c },
	}
	if err := consumeClinepassSSE(strings.NewReader(body), cb); err != nil {
		t.Fatalf("consume: %v", err)
	}
	if got.String() != "Hello world" {
		t.Fatalf("text = %q", got.String())
	}
	if gotStop != "end_turn" {
		t.Fatalf("stop = %q", gotStop)
	}
	if gotP != 11 || gotC != 2 {
		t.Fatalf("usage = (%d,%d)", gotP, gotC)
	}
}

// header + end-to-end round-trip -----------------------------------------------

func TestCallOpenAIRoundTripStreamAndNonStream(t *testing.T) {
	var lastAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastAuth = r.Header.Get("Authorization")
		if r.Header.Get("X-Title") != "Cline" {
			t.Errorf("missing X-Title header")
		}
		if r.Header.Get("X-PLATFORM") != "go" {
			t.Errorf("missing X-PLATFORM header")
		}
		if r.Header.Get("User-Agent") != clinepassAuthUserAgent {
			t.Errorf("wrong User-Agent: %q", r.Header.Get("User-Agent"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if body["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hi\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":1,\"total_tokens\":3}}\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"choices": []map[string]any{{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "ok"},
					"finish_reason": "stop",
				}},
				"usage": map[string]any{"prompt_tokens": 7, "completion_tokens": 4, "total_tokens": 11},
			},
		})
	}))
	defer srv.Close()

	// Redirect the package-level base URL to the test server.
	prevBase := clinepassBaseURL
	clinepassBaseURL = srv.URL
	defer func() { clinepassBaseURL = prevBase }()

	acc := &config.Account{Provider: "clinepass", AccessToken: "tok-xyz"}
	req := &providers.OpenAIRequest{Model: "claude-sonnet-test", Stream: false}

	var gotTxt strings.Builder
	var gotP, gotC int
	cb := &providers.StreamCallback{
		OnText:     func(s string, _ bool) { gotTxt.WriteString(s) },
		OnComplete: func(p, c int) { gotP, gotC = p, c },
	}

	// Non-stream call uses REST client; redirect through providers package.
	restore := providers.SwapClientsForTest(srv.Client(), srv.Client())
	defer restore()

	if err := CallOpenAI(acc, req, cb); err != nil {
		t.Fatalf("non-stream CallOpenAI: %v", err)
	}
	if gotTxt.String() != "ok" {
		t.Fatalf("non-stream text = %q", gotTxt.String())
	}
	if gotP != 7 || gotC != 4 {
		t.Fatalf("non-stream usage = (%d,%d)", gotP, gotC)
	}
	if lastAuth != "Bearer tok-xyz" {
		t.Fatalf("non-stream auth header = %q", lastAuth)
	}

	// Streaming call.
	gotTxt.Reset()
	gotP, gotC = 0, 0
	req.Stream = true
	if err := CallOpenAI(acc, req, cb); err != nil {
		t.Fatalf("stream CallOpenAI: %v", err)
	}
	if gotTxt.String() != "hi" {
		t.Fatalf("stream text = %q", gotTxt.String())
	}
	if gotP != 2 || gotC != 1 {
		t.Fatalf("stream usage = (%d,%d)", gotP, gotC)
	}
}

// Sanity: NormaliseStopReason contract.
func TestNormaliseStopReason(t *testing.T) {
	cases := map[string]string{
		"":           "end_turn",
		"stop":       "end_turn",
		"tool_calls": "tool_use",
		"length":     "max_tokens",
		"unknown":    "unknown",
	}
	for in, want := range cases {
		if got := normaliseStopReason(in); got != want {
			t.Fatalf("normaliseStopReason(%q) = %q want %q", in, got, want)
		}
	}
	_ = context.Background
}
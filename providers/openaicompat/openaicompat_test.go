package openaicompat

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hekato-go/config"
	"hekato-go/providers"
)

// TestCallOpenAINonStreamRoundTrip exercises the OpenAI-compat non-stream
// code path against an httptest server that mimics an OpenAI-shaped vendor.
func TestCallOpenAINonStreamRoundTrip(t *testing.T) {
	var gotAuth string
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
			"choices": [
				{"index":0, "finish_reason":"stop", "message": {"role":"assistant", "content":"hi from openai-compat"}}
			],
			"usage": {"prompt_tokens": 3, "completion_tokens": 4}
		}`)
	}))
	defer srv.Close()

	acc := &config.Account{
		ID:           "test-acc",
		BaseURL:      srv.URL,
		CompatAPIKey: "sk-test",
	}
	req := &providers.OpenAIRequest{
		Model:    "gpt-test",
		Messages: []providers.OpenAIMessage{{Role: "user", Content: "hello"}},
		Stream:   false,
	}

	var text, stop string
	var inT, outT int
	cb := &providers.StreamCallback{
		OnText:       func(s string, _ bool) { text += s },
		OnStopReason: func(s string) { stop = s },
		OnComplete:   func(i, o int) { inT, outT = i, o },
	}
	if err := CallOpenAI(acc, req, cb); err != nil {
		t.Fatalf("CallOpenAI: %v", err)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer sk-test")
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("path = %q, want %q", gotPath, "/chat/completions")
	}
	if text != "hi from openai-compat" {
		t.Fatalf("text = %q", text)
	}
	if stop != "end_turn" {
		t.Fatalf("stop = %q, want end_turn", stop)
	}
	if inT != 3 || outT != 4 {
		t.Fatalf("usage = (%d,%d), want (3,4)", inT, outT)
	}
}

// TestCallOpenAISSE exercises the streaming path.
func TestCallOpenAISSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		_, _ = io.WriteString(w, `data: {"choices":[{"delta":{"content":"hello "}}]}`+"\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = io.WriteString(w, `data: {"choices":[{"delta":{"content":"world"},"finish_reason":"stop"}]}`+"\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = io.WriteString(w, `data: {"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2}}`+"\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	acc := &config.Account{ID: "test", BaseURL: srv.URL, CompatAPIKey: "sk"}
	req := &providers.OpenAIRequest{Model: "m", Messages: []providers.OpenAIMessage{{Role: "user", Content: "hi"}}, Stream: true}

	var text string
	var inT, outT int
	cb := &providers.StreamCallback{
		OnText:     func(s string, _ bool) { text += s },
		OnComplete: func(i, o int) { inT, outT = i, o },
	}
	if err := CallOpenAI(acc, req, cb); err != nil {
		t.Fatalf("CallOpenAI: %v", err)
	}
	if text != "hello world" {
		t.Fatalf("text = %q", text)
	}
	if inT != 1 || outT != 2 {
		t.Fatalf("usage = (%d,%d), want (1,2)", inT, outT)
	}
}

// TestModelsForAccount exercises the live /models endpoint plus ExtraModels.
func TestModelsForAccount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "gpt-test"},
				{"id": "gpt-other"},
				{"id": ""}, // empty id should be filtered
			},
		})
	}))
	defer srv.Close()

	acc := &config.Account{
		ID: "test", BaseURL: srv.URL, CompatAPIKey: "sk",
		ExtraModels: []string{"gpt-other", "manual-model"},
	}
	models := ModelsForAccount(acc)
	ids := []string{}
	for _, m := range models {
		ids = append(ids, m.ModelId)
	}
	want := []string{"gpt-test", "gpt-other", "manual-model"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("models = %v, want %v", ids, want)
	}
}

// TestCallOpenAIMissingConfig confirms misconfiguration errors out cleanly.
func TestCallOpenAIMissingConfig(t *testing.T) {
	if err := CallOpenAI(&config.Account{ID: "x"}, &providers.OpenAIRequest{}, &providers.StreamCallback{}); err == nil ||
		!strings.Contains(err.Error(), "no base URL") {
		t.Fatalf("expected base URL error, got %v", err)
	}
	if err := CallOpenAI(&config.Account{ID: "x", BaseURL: "http://x"}, &providers.OpenAIRequest{}, &providers.StreamCallback{}); err == nil ||
		!strings.Contains(err.Error(), "no API key") {
		t.Fatalf("expected API key error, got %v", err)
	}
}
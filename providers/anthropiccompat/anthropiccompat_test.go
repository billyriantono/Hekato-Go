package anthropiccompat

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

// TestCallMessagesNonStream exercises the Anthropic-compat non-streaming
// code path against a mock server.
func TestCallMessagesNonStream(t *testing.T) {
	var gotAPIKey, gotVersion, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
			"type": "message",
			"content": [
				{"type": "text", "text": "hello from anthropic-compat"}
			],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 5, "output_tokens": 8}
		}`)
	}))
	defer srv.Close()

	acc := &config.Account{
		ID:           "test",
		BaseURL:      srv.URL,
		CompatAPIKey: "ant-test",
	}
	req := &MessagesRequest{
		Model:     "claude-test",
		MaxTokens: 100,
		System:    "be helpful",
		Messages: []MessagesMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
	}

	var text, stop string
	var inT, outT int
	cb := &providers.StreamCallback{
		OnText:       func(s string, _ bool) { text += s },
		OnStopReason: func(s string) { stop = s },
		OnComplete:   func(i, o int) { inT, outT = i, o },
	}
	if err := CallMessages(acc, req, cb); err != nil {
		t.Fatalf("CallMessages: %v", err)
	}
	if gotAPIKey != "ant-test" {
		t.Fatalf("x-api-key = %q", gotAPIKey)
	}
	if gotVersion != anthropicVersion {
		t.Fatalf("anthropic-version = %q", gotVersion)
	}
	if gotPath != "/v1/messages" {
		t.Fatalf("path = %q, want /v1/messages", gotPath)
	}
	if text != "hello from anthropic-compat" {
		t.Fatalf("text = %q", text)
	}
	if stop != "end_turn" {
		t.Fatalf("stop = %q", stop)
	}
	if inT != 5 || outT != 8 {
		t.Fatalf("usage = (%d,%d), want (5,8)", inT, outT)
	}
}

// TestCallMessagesSSE exercises the streaming path.
func TestCallMessagesSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		events := []string{
			`event: message_start` + "\n" + `data: {"type":"message_start","message":{"type":"message","usage":{"input_tokens":3,"output_tokens":0}}}`,
			`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello "}}`,
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}`,
			`event: content_block_stop` + "\n" + `data: {"type":"content_block_stop","index":0}`,
			`event: message_delta` + "\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`,
			`event: message_stop` + "\n" + `data: {"type":"message_stop"}`,
		}
		for _, ev := range events {
			_, _ = io.WriteString(w, ev+"\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer srv.Close()

	acc := &config.Account{ID: "test", BaseURL: srv.URL, CompatAPIKey: "sk"}
	req := &MessagesRequest{
		Model:     "m",
		MaxTokens: 100,
		Stream:    true,
		Messages:  []MessagesMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
	}

	var text string
	var inT, outT int
	cb := &providers.StreamCallback{
		OnText:     func(s string, _ bool) { text += s },
		OnComplete: func(i, o int) { inT, outT = i, o },
	}
	if err := CallMessages(acc, req, cb); err != nil {
		t.Fatalf("CallMessages: %v", err)
	}
	if text != "hello world" {
		t.Fatalf("text = %q", text)
	}
	if inT != 3 || outT != 7 {
		t.Fatalf("usage = (%d,%d), want (3,7)", inT, outT)
	}
}

// TestModelsForAccount exercises the /v1/models endpoint plus ExtraModels.
func TestModelsForAccount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "claude-3-sonnet"},
				{"id": "claude-3-haiku"},
			},
		})
	}))
	defer srv.Close()

	acc := &config.Account{
		ID: "test", BaseURL: srv.URL, CompatAPIKey: "sk",
		ExtraModels: []string{"claude-3-haiku", "custom-model"},
	}
	models := ModelsForAccount(acc)
	ids := []string{}
	for _, m := range models {
		ids = append(ids, m.ModelId)
	}
	want := "claude-3-sonnet,claude-3-haiku,custom-model"
	if strings.Join(ids, ",") != want {
		t.Fatalf("models = %v, want %s", ids, want)
	}
}

// TestCallMessagesMissingConfig exercises early errors.
func TestCallMessagesMissingConfig(t *testing.T) {
	if err := CallMessages(&config.Account{ID: "x"}, &MessagesRequest{}, &providers.StreamCallback{}); err == nil ||
		!strings.Contains(err.Error(), "no base URL") {
		t.Fatalf("expected base URL error, got %v", err)
	}
	if err := CallMessages(&config.Account{ID: "x", BaseURL: "http://x"}, &MessagesRequest{}, &providers.StreamCallback{}); err == nil ||
		!strings.Contains(err.Error(), "no API key") {
		t.Fatalf("expected API key error, got %v", err)
	}
}
package proxy

import (
	"fmt"
	"hekato-go/config"
	accountpool "hekato-go/pool"
	"hekato-go/providers"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// An upstream that dies mid-stream must still leave the client a closed SSE
// stream. Without the terminal chunk a strict client ("stream closed before a
// finish_reason was received") throws away the partial answer it already
// rendered — which is what production was doing against slow reasoning models.
func TestOpenAIStreamClosesAfterUpstreamDropsMidStream(t *testing.T) {
	if err := config.Init(t.TempDir() + "/config.json"); err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f := w.(http.Flusher)
		for _, word := range []string{"partial ", "answer "} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":%q},\"finish_reason\":null}]}\n\n", word)
			f.Flush()
		}
		// Drop the connection without [DONE] or a finish_reason — the shape of
		// a reasoning model whose upstream connection is reset mid-answer.
		panic(http.ErrAbortHandler)
	}))
	defer upstream.Close()

	if err := config.AddAccount(config.Account{
		ID:             "compat-1",
		Email:          "compat@test",
		Enabled:        true,
		ProviderKind:   string(config.ProviderOpenAICompat),
		CompatProtocol: string(config.ProviderOpenAICompat),
		BaseURL:        upstream.URL,
		CompatAPIKey:   "k",
	}); err != nil {
		t.Fatal(err)
	}
	p := accountpool.GetPool()
	p.Reload()
	p.SetModelList("compat-1", []string{"test-model"})
	t.Cleanup(func() { p.SetModelList("compat-1", nil) })

	h := &Handler{pool: p, promptCache: newPromptCacheTracker(defaultPromptCacheTTL), autoRouter: newAutoRouter()}
	req := &OpenAIRequest{Model: "test-model", Stream: true}
	req.Messages = []providers.OpenAIMessage{{Role: "user", Content: "hello"}}

	rec := httptest.NewRecorder()
	h.handleOpenAIStream(rec, req, "test-model", false, 10, "", "")

	body := rec.Body.String()
	if !strings.Contains(body, "partial ") {
		t.Fatalf("the partial answer should reach the client, got:\n%s", body)
	}
	if !strings.Contains(body, `"finish_reason":"length"`) {
		t.Errorf("truncated stream must end with a terminal chunk, got:\n%s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Errorf("truncated stream must be closed with [DONE], got:\n%s", body)
	}
}

// Failover must still work when the upstream fails before saying anything:
// nothing has been sent, so a second account can answer cleanly.
func TestOpenAIStreamFailsOverWhenUpstreamSaysNothing(t *testing.T) {
	if err := config.Init(t.TempDir() + "/config.json"); err != nil {
		t.Fatal(err)
	}

	var hits int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			http.Error(w, "upstream exploded", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"second account answered\"},\"finish_reason\":null}]}\n\n")
		f.Flush()
		fmt.Fprint(w, "data: [DONE]\n\n")
		f.Flush()
	}))
	defer upstream.Close()

	for _, id := range []string{"compat-a", "compat-b"} {
		if err := config.AddAccount(config.Account{
			ID: id, Email: id + "@test", Enabled: true,
			ProviderKind:   string(config.ProviderOpenAICompat),
			CompatProtocol: string(config.ProviderOpenAICompat),
			BaseURL:        upstream.URL, CompatAPIKey: "k",
		}); err != nil {
			t.Fatal(err)
		}
	}
	p := accountpool.GetPool()
	p.Reload()
	p.SetModelList("compat-a", []string{"test-model"})
	p.SetModelList("compat-b", []string{"test-model"})
	t.Cleanup(func() { p.SetModelList("compat-a", nil); p.SetModelList("compat-b", nil) })

	h := &Handler{pool: p, promptCache: newPromptCacheTracker(defaultPromptCacheTTL), autoRouter: newAutoRouter()}
	req := &OpenAIRequest{Model: "test-model", Stream: true}
	req.Messages = []providers.OpenAIMessage{{Role: "user", Content: "hello"}}

	rec := httptest.NewRecorder()
	h.handleOpenAIStream(rec, req, "test-model", false, 10, "", "")

	body := rec.Body.String()
	if hits < 2 {
		t.Fatalf("expected a retry on the second account, upstream saw %d request(s)", hits)
	}
	if !strings.Contains(body, "second account answered") {
		t.Errorf("failover answer missing, got:\n%s", body)
	}
	if !strings.Contains(body, `"finish_reason":"stop"`) {
		t.Errorf("a successful failover must end normally, got:\n%s", body)
	}
}

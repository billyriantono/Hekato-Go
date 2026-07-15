package proxy

import (
	"encoding/json"
	"kiro-go/config"
	"kiro-go/providers/grok"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallOpenAIUpstreamRoutesGrokWithoutKiro(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer grok-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		var req ResponsesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "grok-4.5" {
			t.Fatalf("model = %q, want grok-4.5", req.Model)
		}
		_ = json.NewEncoder(w).Encode(ResponsesObject{
			Status: "completed",
			Output: []ResponseOutputItem{{Type: "message", Content: []ResponseContentPart{{Type: "output_text", Text: "OK"}}}},
			Usage:  ResponsesUsage{InputTokens: 2, OutputTokens: 1, TotalTokens: 3},
		})
	}))
	defer server.Close()

	old := grok.ResponsesEndpoint
	grok.ResponsesEndpoint = server.URL
	defer func() { grok.ResponsesEndpoint = old }()

	var text string
	var inTokens, outTokens int
	err := CallOpenAIUpstreamAPI(
		&config.Account{AuthMethod: "grok", Provider: "grok", AccessToken: "grok-token"},
		&OpenAIRequest{Model: "grok-4.5-high", Messages: []OpenAIMessage{{Role: "user", Content: "say ok"}}},
		false,
		&KiroStreamCallback{
			OnText:     func(s string, _ bool) { text += s },
			OnComplete: func(in, out int) { inTokens, outTokens = in, out },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if text != "OK" || inTokens != 2 || outTokens != 1 {
		t.Fatalf("text=%q tokens=%d/%d", text, inTokens, outTokens)
	}
}

func TestRefreshAccountInfoRoutesGrokWithoutKiro(t *testing.T) {
	billing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]float64{"credits_remaining": 75, "credits_limit": 100})
	}))
	defer billing.Close()
	user := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"email":        "grok@example.com",
			"id":           "user-1",
			"subscription": map[string]string{"name": "SuperGrok"},
		})
	}))
	defer user.Close()

	oldBilling, oldUser := grok.BillingEndpoint, grok.UserEndpoint
	grok.BillingEndpoint, grok.UserEndpoint = billing.URL, user.URL
	defer func() { grok.BillingEndpoint, grok.UserEndpoint = oldBilling, oldUser }()

	info, err := RefreshAccountInfo(&config.Account{AuthMethod: "grok", Provider: "grok", AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	if info.Email != "grok@example.com" || info.SubscriptionTitle != "SuperGrok" || info.UsageCurrent != 25 || info.UsageLimit != 100 {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestUnknownProviderDoesNotFallThroughToKiro(t *testing.T) {
	err := CallOpenAIUpstreamAPI(&config.Account{AuthMethod: "oauth", Provider: "future-provider"}, &OpenAIRequest{}, false, nil)
	if err == nil {
		t.Fatal("expected unsupported provider error")
	}
}

// TestClaudeInterfaceRoutesToGrok proves a Claude-format request (/v1/messages
// shape) can drive a Grok account: Grok now exposes claudeChat, which runs
// ClaudeToNeutral → NeutralToOpenAI → Grok's OpenAI adapter.
func TestClaudeInterfaceRoutesToGrok(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ResponsesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "grok-4.5" {
			t.Fatalf("model = %q, want grok-4.5", req.Model)
		}
		_ = json.NewEncoder(w).Encode(ResponsesObject{
			Status: "completed",
			Output: []ResponseOutputItem{{Type: "message", Content: []ResponseContentPart{{Type: "output_text", Text: "salut"}}}},
			Usage:  ResponsesUsage{InputTokens: 3, OutputTokens: 2, TotalTokens: 5},
		})
	}))
	defer server.Close()

	old := grok.ResponsesEndpoint
	grok.ResponsesEndpoint = server.URL
	defer func() { grok.ResponsesEndpoint = old }()

	var text string
	err := CallClaudeUpstreamAPI(
		&config.Account{AuthMethod: "grok", Provider: "grok", AccessToken: "grok-token"},
		&ClaudeRequest{Model: "grok-4.5", Messages: []ClaudeMessage{{Role: "user", Content: "dis bonjour"}}},
		false,
		&KiroStreamCallback{OnText: func(s string, _ bool) { text += s }},
	)
	if err != nil {
		t.Fatal(err)
	}
	if text != "salut" {
		t.Fatalf("text = %q, want salut", text)
	}
}

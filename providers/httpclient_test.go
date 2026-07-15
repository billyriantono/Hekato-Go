package providers

import (
	"io"
	"kiro-go/config"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestBuildKiroTransportUsesExplicitProxyURL(t *testing.T) {
	transport := buildTransport("http://proxy.local:8080")
	req := &http.Request{URL: mustParseURL(t, "https://q.us-east-1.amazonaws.com")}

	got, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("unexpected proxy error: %v", err)
	}
	assertProxyURL(t, got, "http://proxy.local:8080")
}

func TestBuildKiroTransportFallsBackToEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://env-proxy.local:2323")
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	transport := buildTransport("")
	req := &http.Request{URL: mustParseURL(t, "https://q.us-east-1.amazonaws.com")}

	got, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("unexpected proxy error: %v", err)
	}
	assertProxyURL(t, got, "http://env-proxy.local:2323")
}

func TestAccountRelayOverridesGlobalOutbound(t *testing.T) {
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Relay-Target") != "https://q.us-east-1.amazonaws.com/test" || r.Header.Get("X-Relay-Key") != "account-secret" {
			t.Fatalf("unexpected relay headers: %q/%q", r.Header.Get("X-Relay-Target"), r.Header.Get("X-Relay-Key"))
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer relay.Close()

	resp, err := GetRestClientForAccount(&config.Account{RelayURL: relay.URL, RelaySecret: "account-secret"}).Get("https://q.us-east-1.amazonaws.com/test")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if body, _ := io.ReadAll(resp.Body); string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}
}

func TestInitHTTPClientsKeepsShortRestTimeout(t *testing.T) {
	InitHTTPClients("")
	t.Cleanup(func() { InitHTTPClients("") })

	streamClient := streamClientStore.Load()
	restClient := restClientStore.Load()

	if streamClient.Timeout != 5*time.Minute {
		t.Fatalf("expected streaming timeout to be 5m, got %s", streamClient.Timeout)
	}
	if restClient.Timeout != 30*time.Second {
		t.Fatalf("expected REST timeout to stay 30s, got %s", restClient.Timeout)
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("invalid test URL: %v", err)
	}
	return parsed
}

func assertProxyURL(t *testing.T, got *url.URL, want string) {
	t.Helper()
	if got == nil {
		t.Fatalf("expected proxy URL %q, got nil", want)
	}
	if got.String() != want {
		t.Fatalf("expected proxy URL %q, got %q", want, got.String())
	}
}

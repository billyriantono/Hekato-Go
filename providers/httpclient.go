// Package providers holds the shared contract between the proxy core and the
// per-provider packages (providers/kiro, providers/codebuddy, providers/grok):
// the neutral request representation (NeutralChat), wire types, upstream error type, outbound HTTP
// clients, and the admin route registry.
package providers

import (
	"fmt"
	"kiro-go/config"
	"kiro-go/egress"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// Global HTTP clients, swappable at runtime to apply proxy reconfiguration without restart.
var streamClientStore atomic.Pointer[http.Client]
var restClientStore atomic.Pointer[http.Client]

// proxyClientCache caches http.Client instances keyed by proxy URL for per-account proxy support.
var proxyClientCache sync.Map

func init() {
	InitHTTPClients("")
}

func clientForAccount(account *config.Account, rest bool) *http.Client {
	if account == nil || (account.ProxyURL == "" && account.RelayURL == "") {
		if rest {
			return restClientStore.Load()
		}
		return streamClientStore.Load()
	}
	key := fmt.Sprintf("%t\x00%s\x00%s\x00%s", rest, account.ProxyURL, account.RelayURL, account.RelaySecret)
	if cached, ok := proxyClientCache.Load(key); ok {
		return cached.(*http.Client)
	}
	transport := http.RoundTripper(buildTransport(account.ProxyURL))
	if account.RelayURL != "" {
		transport = egress.NewRelayTransportWith(transport, account.RelayURL, account.RelaySecret)
	}
	timeout := 5 * time.Minute
	if rest {
		timeout = 30 * time.Second
	}
	client := &http.Client{Timeout: timeout, Transport: transport}
	proxyClientCache.Store(key, client)
	return client
}

func GetClientForAccount(account *config.Account) *http.Client {
	return clientForAccount(account, false)
}
func GetRestClientForAccount(account *config.Account) *http.Client {
	return clientForAccount(account, true)
}

// buildTransport constructs an HTTP Transport with optional outbound proxy support.
func buildTransport(proxyURL string) *http.Transport {
	t := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	}
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			t.Proxy = http.ProxyURL(u)
			// Proxied connections cannot negotiate HTTP/2.
			t.ForceAttemptHTTP2 = false
		}
	} else {
		t.Proxy = http.ProxyFromEnvironment
	}
	return t
}

// InitHTTPClients initializes (or reinitializes) the HTTP clients used for Kiro API requests.
func InitHTTPClients(proxyURL string) {
	client := &http.Client{
		Timeout:   5 * time.Minute,
		Transport: egress.NewRelayTransport(buildTransport(proxyURL)),
	}
	streamClientStore.Store(client)

	restClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: egress.NewRelayTransport(buildTransport(proxyURL)),
	}
	restClientStore.Store(restClient)
}

// SwapClientsForTest replaces the default streaming/REST clients and returns a
// restore func. Test-only seam for handler-level integration tests.
func SwapClientsForTest(streaming, rest *http.Client) (restore func()) {
	oldStreaming, oldRest := streamClientStore.Load(), restClientStore.Load()
	if streaming != nil {
		streamClientStore.Store(streaming)
	}
	if rest != nil {
		restClientStore.Store(rest)
	}
	return func() {
		streamClientStore.Store(oldStreaming)
		restClientStore.Store(oldRest)
	}
}

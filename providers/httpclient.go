// Package providers holds the shared contract between the proxy core and the
// per-provider packages (providers/kiro, providers/codebuddy, providers/grok):
// the internal request IR, wire types, upstream error type, outbound HTTP
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
var kiroHttpStore atomic.Pointer[http.Client]
var kiroRestHttpStore atomic.Pointer[http.Client]

// proxyClientCache caches http.Client instances keyed by proxy URL for per-account proxy support.
var proxyClientCache sync.Map

func init() {
	InitKiroHttpClient("")
}

func clientForAccount(account *config.Account, rest bool) *http.Client {
	if account == nil || (account.ProxyURL == "" && account.RelayURL == "") {
		if rest {
			return kiroRestHttpStore.Load()
		}
		return kiroHttpStore.Load()
	}
	key := fmt.Sprintf("%t\x00%s\x00%s\x00%s", rest, account.ProxyURL, account.RelayURL, account.RelaySecret)
	if cached, ok := proxyClientCache.Load(key); ok {
		return cached.(*http.Client)
	}
	transport := http.RoundTripper(buildKiroTransport(account.ProxyURL))
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

// buildKiroTransport constructs an HTTP Transport with optional outbound proxy support.
func buildKiroTransport(proxyURL string) *http.Transport {
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

// InitKiroHttpClient initializes (or reinitializes) the HTTP clients used for Kiro API requests.
func InitKiroHttpClient(proxyURL string) {
	client := &http.Client{
		Timeout:   5 * time.Minute,
		Transport: egress.NewRelayTransport(buildKiroTransport(proxyURL)),
	}
	kiroHttpStore.Store(client)

	restClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: egress.NewRelayTransport(buildKiroTransport(proxyURL)),
	}
	kiroRestHttpStore.Store(restClient)
}

// SwapClientsForTest replaces the default streaming/REST clients and returns a
// restore func. Test-only seam for handler-level integration tests.
func SwapClientsForTest(streaming, rest *http.Client) (restore func()) {
	oldStreaming, oldRest := kiroHttpStore.Load(), kiroRestHttpStore.Load()
	if streaming != nil {
		kiroHttpStore.Store(streaming)
	}
	if rest != nil {
		kiroRestHttpStore.Store(rest)
	}
	return func() {
		kiroHttpStore.Store(oldStreaming)
		kiroRestHttpStore.Store(oldRest)
	}
}

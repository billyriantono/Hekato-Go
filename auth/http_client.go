// Package auth 提供认证相关功能的 HTTP 客户端
package auth

import (
	"kiro-go/config"
	"kiro-go/egress"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// 全局 HTTP 客户端存储，支持运行时代理重配置
var httpClientStore atomic.Pointer[http.Client]

// authProxyClientCache caches per-proxy auth HTTP clients.
var authProxyClientCache sync.Map

// httpClient 返回当前全局 auth HTTP 客户端
func httpClient() *http.Client {
	return httpClientStore.Load()
}

func init() {
	InitHttpClient("")
}

// GetAuthClientForProxy returns an auth client with a fixed proxy (used by SSO flows).
func GetAuthClientForProxy(proxyURL string) *http.Client {
	if proxyURL == "" {
		return httpClient()
	}
	if cached, ok := authProxyClientCache.Load(proxyURL); ok {
		return cached.(*http.Client)
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: egress.NewRelayTransport(buildAuthTransport(proxyURL))}
	authProxyClientCache.Store(proxyURL, client)
	return client
}

// GetAuthClientForAccount applies the account relay/proxy override, or inherits global settings.
func GetAuthClientForAccount(account *config.Account) *http.Client {
	if account == nil || (account.ProxyURL == "" && account.RelayURL == "") {
		return httpClient()
	}
	key := account.ProxyURL + "\x00" + account.RelayURL + "\x00" + account.RelaySecret
	if cached, ok := authProxyClientCache.Load(key); ok {
		return cached.(*http.Client)
	}
	transport := http.RoundTripper(buildAuthTransport(account.ProxyURL))
	if account.RelayURL != "" {
		transport = egress.NewRelayTransportWith(transport, account.RelayURL, account.RelaySecret)
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: transport}
	authProxyClientCache.Store(key, client)
	return client
}

// buildAuthTransport 构建带可选代理的 Transport
func buildAuthTransport(proxyURL string) *http.Transport {
	t := &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	}
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			t.Proxy = http.ProxyURL(u)
			t.ForceAttemptHTTP2 = false
		}
	} else {
		t.Proxy = http.ProxyFromEnvironment
	}
	return t
}

// InitHttpClient 初始化（或重新初始化）auth 模块的全局 HTTP 客户端
func InitHttpClient(proxyURL string) {
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: egress.NewRelayTransport(buildAuthTransport(proxyURL)),
	}
	httpClientStore.Store(client)
}

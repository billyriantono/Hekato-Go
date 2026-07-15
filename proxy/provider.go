// Composition root for provider routing: wires each provider package's
// transport to the shared translators and exposes the capability-checked
// adapter table used by the handlers and the account pool filter.
package proxy

import (
	"fmt"
	"kiro-go/config"
	"kiro-go/pool"
	"kiro-go/providers"
	"kiro-go/providers/codebuddy"
	"kiro-go/providers/grok"
	"kiro-go/providers/kiro"
	"net/http"
)

// Local aliases keep provider code compact while config.ProviderForAccount is
// the single classification source shared by auth and proxy.
type providerKind = config.AccountProvider

const (
	providerKiro      = config.ProviderKiro
	providerCodeBuddy = config.ProviderCodeBuddy
	providerGrok      = config.ProviderGrok
)

// providerAdapter is the one registration point for proxy capabilities. Adding
// a provider no longer changes handlers: register supported operations here;
// missing capabilities return a clear error instead of falling through to Kiro.
type providerAdapter struct {
	kind            providerKind
	claudeChat      func(*config.Account, *ClaudeRequest, bool, *KiroStreamCallback) error
	openAIChat      func(*config.Account, *OpenAIRequest, bool, *KiroStreamCallback) error
	nativeResponses func(http.ResponseWriter, http.Flusher, *config.Account, *ResponsesRequest) error
	models          func(*config.Account) ([]ModelInfo, error)
	usage           func(*config.Account) (*config.AccountInfo, error)
}

var providerAdapters = map[providerKind]providerAdapter{
	providerKiro: {
		kind: providerKiro,
		claudeChat: func(a *config.Account, r *ClaudeRequest, thinking bool, cb *KiroStreamCallback) error {
			return kiro.CallAPI(a, ClaudeToKiro(r, thinking), cb)
		},
		openAIChat: func(a *config.Account, r *OpenAIRequest, thinking bool, cb *KiroStreamCallback) error {
			return kiro.CallAPI(a, OpenAIToKiro(r, thinking), cb)
		},
		models: kiro.ListModels,
		usage:  kiro.RefreshAccountInfo,
	},
	providerCodeBuddy: {
		kind: providerCodeBuddy,
		claudeChat: func(a *config.Account, r *ClaudeRequest, thinking bool, cb *KiroStreamCallback) error {
			return codebuddy.Call(a, ClaudeToCodeBuddy(r, thinking), cb)
		},
		openAIChat: func(a *config.Account, r *OpenAIRequest, thinking bool, cb *KiroStreamCallback) error {
			return codebuddy.Call(a, OpenAIToCodeBuddy(r, thinking), cb)
		},
		models: func(a *config.Account) ([]ModelInfo, error) { return codebuddy.ModelsForAccount(a), nil },
		usage:  codebuddy.FetchUsage,
	},
	providerGrok: {
		kind: providerGrok,
		openAIChat: func(a *config.Account, r *OpenAIRequest, _ bool, cb *KiroStreamCallback) error {
			return grok.CallOpenAI(a, r, cb)
		},
		nativeResponses: grok.CallUpstream,
		models:          func(a *config.Account) ([]ModelInfo, error) { return grok.RefreshModels(a), nil },
		usage:           grok.FetchUsage,
	},
}

func providerForAccount(account *config.Account) (providerKind, error) {
	return config.ProviderForAccount(account)
}

func adapterForAccount(account *config.Account) (providerAdapter, error) {
	kind, err := providerForAccount(account)
	if err != nil {
		return providerAdapter{}, err
	}
	adapter, ok := providerAdapters[kind]
	if !ok {
		return providerAdapter{}, fmt.Errorf("provider %q is classified but not registered", kind)
	}
	return adapter, nil
}

// providerCapability names the adapter entry an endpoint needs. Pool selection
// filters on it so an account whose provider cannot serve the endpoint (e.g. a
// Grok account for /v1/messages) never gets picked and never burns a retry.
type providerCapability int

const (
	capClaudeChat providerCapability = iota
	capOpenAIChat
	capResponses // native Responses transport or the OpenAI-chat fallback
)

func (a providerAdapter) supports(cap providerCapability) bool {
	switch cap {
	case capClaudeChat:
		return a.claudeChat != nil
	case capOpenAIChat:
		return a.openAIChat != nil
	case capResponses:
		return a.nativeResponses != nil || a.openAIChat != nil
	}
	return false
}

func capabilityFilter(cap providerCapability) pool.AccountFilter {
	return func(acc *config.Account) bool {
		adapter, err := adapterForAccount(acc)
		return err == nil && adapter.supports(cap)
	}
}

func mustBeProvider(account *config.Account, want providerKind) bool {
	got, err := providerForAccount(account)
	return err == nil && got == want
}

func unsupportedProviderCapability(kind providerKind, capability string) error {
	return fmt.Errorf("provider %s does not support %s", kind, capability)
}

// ListAvailableModels dispatches model discovery through the provider adapter.
func ListAvailableModels(account *config.Account) ([]ModelInfo, error) {
	adapter, err := adapterForAccount(account)
	if err != nil {
		return nil, err
	}
	if adapter.models == nil {
		return nil, unsupportedProviderCapability(adapter.kind, "model discovery")
	}
	return adapter.models(account)
}

// RefreshAccountInfo dispatches usage refresh through the provider adapter.
func RefreshAccountInfo(account *config.Account) (*config.AccountInfo, error) {
	adapter, err := adapterForAccount(account)
	if err != nil {
		return nil, err
	}
	if adapter.usage == nil {
		return nil, unsupportedProviderCapability(adapter.kind, "usage refresh")
	}
	return adapter.usage(account)
}

// Host implementation handed to provider admin routes (providers.Host).
func (h *Handler) ReloadPool() { h.pool.Reload() }

func (h *Handler) RefreshAccountModels(account *config.Account) error {
	return h.fetchAndCacheAccountModels(account)
}

var _ providers.Host = (*Handler)(nil)

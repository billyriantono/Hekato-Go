package proxy

import (
	"fmt"
	"kiro-go/config"
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
	internalChat    func(*config.Account, *KiroPayload, *KiroStreamCallback) error
	claudeChat      func(*config.Account, *ClaudeRequest, bool, *KiroStreamCallback) error
	openAIChat      func(*config.Account, *OpenAIRequest, bool, *KiroStreamCallback) error
	nativeResponses func(http.ResponseWriter, http.Flusher, *config.Account, *ResponsesRequest) error
	models          func(*config.Account) ([]ModelInfo, error)
	usage           func(*config.Account) (*config.AccountInfo, error)
}

var providerAdapters = map[providerKind]providerAdapter{
	providerKiro: {
		kind:         providerKiro,
		internalChat: CallKiroAPI,
		claudeChat: func(a *config.Account, r *ClaudeRequest, thinking bool, cb *KiroStreamCallback) error {
			return CallKiroAPI(a, ClaudeToKiro(r, thinking), cb)
		},
		openAIChat: func(a *config.Account, r *OpenAIRequest, thinking bool, cb *KiroStreamCallback) error {
			return CallKiroAPI(a, OpenAIToKiro(r, thinking), cb)
		},
		models: listKiroModels,
		usage:  refreshKiroAccountInfo,
	},
	providerCodeBuddy: {
		kind:         providerCodeBuddy,
		internalChat: callCodeBuddyChatAPI,
		claudeChat: func(a *config.Account, r *ClaudeRequest, thinking bool, cb *KiroStreamCallback) error {
			return callCodeBuddyChatRequestAPI(a, ClaudeToCodeBuddy(r, thinking), cb)
		},
		openAIChat: func(a *config.Account, r *OpenAIRequest, thinking bool, cb *KiroStreamCallback) error {
			return callCodeBuddyChatRequestAPI(a, OpenAIToCodeBuddy(r, thinking), cb)
		},
		models: func(a *config.Account) ([]ModelInfo, error) { return codeBuddyModelsForAccount(a), nil },
		usage:  FetchCodeBuddyUsage,
	},
	providerGrok: {
		kind: providerGrok,
		openAIChat: func(a *config.Account, r *OpenAIRequest, _ bool, cb *KiroStreamCallback) error {
			return callGrokOpenAIAPI(a, r, cb)
		},
		nativeResponses: CallGrokUpstreamAPI,
		models:          func(a *config.Account) ([]ModelInfo, error) { return refreshGrokModels(a), nil },
		usage:           FetchGrokUsage,
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

func mustBeProvider(account *config.Account, want providerKind) bool {
	got, err := providerForAccount(account)
	return err == nil && got == want
}

func unsupportedProviderCapability(kind providerKind, capability string) error {
	return fmt.Errorf("provider %s does not support %s", kind, capability)
}

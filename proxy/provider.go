// Composition root for provider routing: wires each provider package's
// transport to the shared translators and exposes the capability-checked
// adapter table used by the handlers and the account pool filter.
package proxy

import (
	"fmt"
	"hekato-go/config"
	"hekato-go/pool"
	"hekato-go/providers"
	"net/http"
	"strings"
)

// Local aliases keep provider code compact while config.ProviderForAccount is
// the single classification source shared by auth and proxy.
type providerKind = config.AccountProvider

const (
	providerKiro            = config.ProviderKiro
	providerCodeBuddy       = config.ProviderCodeBuddy
	providerGrok            = config.ProviderGrok
	providerCodex           = config.ProviderCodex
	providerClinepass       = config.ProviderClinepass
	providerOpenAICompat    = config.ProviderOpenAICompat
	providerAnthropicCompat = config.ProviderAnthropicCompat
)

// providerAdapter is the one registration point for proxy capabilities. Each
// provider package is wired in its own provider_<name>.go file via
// registerAdapter from init(); handlers never reference a provider directly.
// Missing capabilities return a clear error instead of falling through.
type providerAdapter struct {
	kind           providerKind
	chatFromClaude func(*config.Account, *ClaudeRequest, bool, *StreamCallback) error
	chatFromOpenAI func(*config.Account, *OpenAIRequest, bool, *StreamCallback) error
	// responses is an optional native Responses-API transport; providers without
	// it are served through chatFromOpenAI.
	responses  func(http.ResponseWriter, http.Flusher, *config.Account, *ResponsesRequest) error
	listModels func(*config.Account) ([]ModelInfo, error)
	fetchUsage func(*config.Account) (*config.AccountInfo, error)
	// staticModels marks listModels as a local catalog (no network), so the
	// pool can be seeded with it synchronously at startup and routing never
	// sees an empty model list for the provider.
	staticModels bool
	// probeModel optionally picks the model for Test / warmup from the
	// account's list (e.g. ClinePass must probe a pass-covered model).
	probeModel func([]ModelInfo) string
}

var providerAdapters = map[providerKind]providerAdapter{}

// registerAdapter is called from each provider_<name>.go init(). Registering
// the same kind twice is a programming error caught at startup.
func registerAdapter(a providerAdapter) {
	if _, dup := providerAdapters[a.kind]; dup {
		panic(fmt.Sprintf("provider adapter registered twice: %s", a.kind))
	}
	providerAdapters[a.kind] = a
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
		return a.chatFromClaude != nil
	case capOpenAIChat:
		return a.chatFromOpenAI != nil
	case capResponses:
		return a.responses != nil || a.chatFromOpenAI != nil
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
	if adapter.listModels == nil {
		return nil, unsupportedProviderCapability(adapter.kind, "model discovery")
	}
	models, err := adapter.listModels(account)
	if err != nil {
		return nil, err
	}
	// Operator-added models for this account (catalog gaps, new releases).
	if account != nil && len(account.ExtraModels) > 0 {
		seen := make(map[string]bool, len(models))
		for _, m := range models {
			seen[strings.ToLower(m.ModelId)] = true
		}
		for _, id := range account.ExtraModels {
			id = strings.TrimSpace(id)
			if id == "" || seen[strings.ToLower(id)] {
				continue
			}
			seen[strings.ToLower(id)] = true
			models = append(models, ModelInfo{ModelId: id, ModelName: id, InputTypes: []string{"text"}})
		}
	}
	return models, nil
}

// RefreshAccountInfo dispatches usage refresh through the provider adapter.
func RefreshAccountInfo(account *config.Account) (*config.AccountInfo, error) {
	adapter, err := adapterForAccount(account)
	if err != nil {
		return nil, err
	}
	if adapter.fetchUsage == nil {
		return nil, unsupportedProviderCapability(adapter.kind, "usage refresh")
	}
	return adapter.fetchUsage(account)
}

// Host implementation handed to provider admin routes (providers.Host).
func (h *Handler) ReloadPool() { h.pool.Reload() }

func (h *Handler) RefreshAccountModels(account *config.Account) error {
	return h.fetchAndCacheAccountModels(account)
}

var _ providers.Host = (*Handler)(nil)

package config

import (
	"fmt"
	"strings"
)

// AccountProvider is the canonical upstream type used across auth and proxy.
type AccountProvider string

const (
	ProviderKiro            AccountProvider = "kiro"
	ProviderCodeBuddy       AccountProvider = "codebuddy"
	ProviderGrok            AccountProvider = "grok"
	ProviderCodex           AccountProvider = "codex"
	ProviderClinepass       AccountProvider = "clinepass"
	ProviderOpenAICompat    AccountProvider = "openai_compat"
	ProviderAnthropicCompat AccountProvider = "anthropic_compat"
	ProviderOpenCodeZen     AccountProvider = "opencode_zen"
	ProviderOpenCodeGo      AccountProvider = "opencode_go"
)

// ProviderForAccount returns the canonical provider for an account. Accounts
// created after the ProviderKind field was introduced carry it explicitly;
// legacy rows fall back to substring classification of the free-text
// AuthMethod/Provider fields. Unknown values fail closed so adding a provider
// cannot accidentally route credentials to Kiro.
func ProviderForAccount(account *Account) (AccountProvider, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	switch AccountProvider(strings.ToLower(strings.TrimSpace(account.ProviderKind))) {
	case ProviderKiro:
		return ProviderKiro, nil
	case ProviderCodeBuddy:
		return ProviderCodeBuddy, nil
	case ProviderGrok:
		return ProviderGrok, nil
	case ProviderCodex:
		return ProviderCodex, nil
	case ProviderClinepass:
		return ProviderClinepass, nil
	case ProviderOpenAICompat:
		return ProviderOpenAICompat, nil
	case ProviderAnthropicCompat:
		return ProviderAnthropicCompat, nil
	case ProviderOpenCodeZen:
		return ProviderOpenCodeZen, nil
	case ProviderOpenCodeGo:
		return ProviderOpenCodeGo, nil
	}
	joined := strings.ToLower(strings.TrimSpace(account.AuthMethod + " " + account.Provider))
	switch {
	case strings.Contains(joined, "openai_compat") || strings.Contains(joined, "openai-compat"):
		return ProviderOpenAICompat, nil
	case strings.Contains(joined, "anthropic_compat") || strings.Contains(joined, "anthropic-compat"):
		return ProviderAnthropicCompat, nil
	case strings.Contains(joined, "opencode_zen") || strings.Contains(joined, "opencode-zen") || strings.Contains(joined, "ocz"):
		return ProviderOpenCodeZen, nil
	case strings.Contains(joined, "opencode_go") || strings.Contains(joined, "opencode-go") || strings.Contains(joined, "ocg"):
		return ProviderOpenCodeGo, nil
	case strings.Contains(joined, "grok") || strings.Contains(joined, "xai") || strings.Contains(joined, "x.ai"):
		return ProviderGrok, nil
	case strings.Contains(joined, "codebuddy"):
		return ProviderCodeBuddy, nil
	case strings.Contains(joined, "clinepass") || strings.Contains(joined, "cline-pass") || strings.Contains(joined, "cline.bot") || strings.Contains(joined, "clp_"):
		return ProviderClinepass, nil
	case strings.Contains(joined, "codex") || strings.Contains(joined, "openai") || strings.Contains(joined, "chatgpt"):
		return ProviderCodex, nil
	case joined == "" || strings.Contains(joined, "kiro") || strings.Contains(joined, "builderid") ||
		strings.Contains(joined, "enterprise") || strings.Contains(joined, "azure") ||
		strings.Contains(joined, "github") || strings.Contains(joined, "google") ||
		strings.Contains(joined, "social") || strings.Contains(joined, "idc") ||
		strings.Contains(joined, "external_idp"):
		return ProviderKiro, nil
	default:
		return "", fmt.Errorf("unsupported account provider: authMethod=%q provider=%q", account.AuthMethod, account.Provider)
	}
}

// StampProviderKind resolves and persists the canonical provider on the
// account struct. Called from AddAccount so every creation path (all auth and
// import flows) stores an explicit provider instead of relying on substring
// classification forever.
func StampProviderKind(account *Account) error {
	kind, err := ProviderForAccount(account)
	if err != nil {
		return err
	}
	account.ProviderKind = string(kind)
	return nil
}
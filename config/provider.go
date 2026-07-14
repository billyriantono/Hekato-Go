package config

import (
	"fmt"
	"strings"
)

// AccountProvider is the canonical upstream type used across auth and proxy.
type AccountProvider string

const (
	ProviderKiro      AccountProvider = "kiro"
	ProviderCodeBuddy AccountProvider = "codebuddy"
	ProviderGrok      AccountProvider = "grok"
)

// ProviderForAccount classifies an account once. Unknown values fail closed so
// adding a provider cannot accidentally route credentials to Kiro.
func ProviderForAccount(account *Account) (AccountProvider, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	joined := strings.ToLower(strings.TrimSpace(account.AuthMethod + " " + account.Provider))
	switch {
	case strings.Contains(joined, "grok") || strings.Contains(joined, "xai") || strings.Contains(joined, "x.ai"):
		return ProviderGrok, nil
	case strings.Contains(joined, "codebuddy"):
		return ProviderCodeBuddy, nil
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

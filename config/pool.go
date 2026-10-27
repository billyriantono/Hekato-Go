package config

import (
	"fmt"
	"net/url"
	"strings"
)

// IsCodeBuddyCN reports whether a CodeBuddy account talks to the China gateway.
// Shared by the CodeBuddy adapter (which endpoint to call) and PoolKey (which
// pool the account belongs to) so the two can never disagree.
func IsCodeBuddyCN(account *Account) bool {
	joined := strings.ToLower(strings.TrimSpace(account.AuthMethod + " " + account.Provider + " " + account.Region))
	return strings.Contains(joined, "cn") || strings.Contains(joined, "china") || strings.Contains(joined, "tencent") || strings.Contains(joined, "copilot")
}

// IsWorkBuddy reports whether a CodeBuddy account talks to the WorkBuddy host
// (www.workbuddy.ai). Same backend and catalog as CodeBuddy Global; only the
// host and X-Domain differ.
func IsWorkBuddy(account *Account) bool {
	joined := strings.ToLower(account.AuthMethod + " " + account.Provider + " " + account.Region)
	return strings.Contains(joined, "workbuddy")
}

// PoolKey groups accounts that reach the same upstream with the same catalog:
// the provider kind, plus a variant suffix only where the upstream really
// differs (CodeBuddy CN). OpenAI/Anthropic-compatible accounts are keyed by
// their vendor host ("openrouter.ai", "localhost-11434"), so several keys for
// one vendor share a pool; "" when the base URL has no host.
func PoolKey(account *Account) string {
	kind, err := ProviderForAccount(account)
	if err != nil {
		return ""
	}
	if kind == ProviderOpenAICompat || kind == ProviderAnthropicCompat {
		u, err := url.Parse(strings.TrimSpace(account.BaseURL))
		if err != nil || u.Hostname() == "" {
			return ""
		}
		key := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
		if p := u.Port(); p != "" {
			key += "-" + p // ":" would clash with the blacklist's "pool:pattern"
		}
		return key
	}
	if kind == ProviderCodeBuddy && IsCodeBuddyCN(account) {
		return "codebuddy-cn"
	}
	return string(kind)
}

// GetPoolPrefixes returns a copy of the custom prefix map (pool key → prefix).
func GetPoolPrefixes() map[string]string {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	out := make(map[string]string, len(cfg.PoolPrefixes))
	for k, v := range cfg.PoolPrefixes {
		out[k] = v
	}
	return out
}

// UpdatePoolPrefixes validates and persists the custom prefix map. Empty
// values are dropped (the pool falls back to its key). A prefix may not
// contain "/", repeat, or shadow another pool's key.
func UpdatePoolPrefixes(m map[string]string) error {
	clean := make(map[string]string, len(m))
	used := make(map[string]string, len(m))
	for k, v := range m {
		k, v = strings.ToLower(strings.TrimSpace(k)), strings.ToLower(strings.TrimSpace(v))
		if k == "" || v == "" || v == k {
			continue
		}
		if !isPoolKey(k) {
			return fmt.Errorf("unknown pool %q", k)
		}
		if strings.Contains(v, "/") || strings.Contains(v, ":") {
			return fmt.Errorf("prefix %q must not contain \"/\" or \":\"", v)
		}
		if isPoolKey(v) || v == "cline-pass" {
			return fmt.Errorf("prefix %q for %s collides with a pool key", v, k)
		}
		if other, dup := used[v]; dup {
			return fmt.Errorf("prefix %q is used by both %s and %s", v, other, k)
		}
		used[v] = k
		clean[k] = v
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.PoolPrefixes = clean
	return Save()
}

// GetPoolTestModel returns the pool's test model, or "" when none is set.
func GetPoolTestModel(pool string) string {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return cfg.PoolTestModels[pool]
}

// GetPoolTestModels returns a copy of the pool test model map.
func GetPoolTestModels() map[string]string {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	out := make(map[string]string, len(cfg.PoolTestModels))
	for k, v := range cfg.PoolTestModels {
		out[k] = v
	}
	return out
}

// UpdatePoolTestModels validates and persists the pool test model map. Empty
// values are dropped (the pool falls back to the global test model).
func UpdatePoolTestModels(m map[string]string) error {
	clean := make(map[string]string, len(m))
	for k, v := range m {
		k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		if !isPoolKey(k) {
			return fmt.Errorf("unknown pool %q", k)
		}
		clean[k] = v
	}
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.PoolTestModels = clean
	return Save()
}

func isPoolKey(s string) bool {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return isPoolKeyLocked(s)
}

// isPoolKeyLocked reports whether s is a fixed provider pool or the key of a
// configured compatible-vendor pool. Caller holds cfgLock.
func isPoolKeyLocked(s string) bool {
	switch AccountProvider(s) {
	case ProviderKiro, ProviderCodeBuddy, ProviderGrok, ProviderCodex, ProviderClinepass,
		ProviderOpenCodeZen, ProviderOpenCodeGo, ProviderCommandCode:
		return true
	}
	if s == "codebuddy-cn" {
		return true
	}
	for i := range cfg.Accounts {
		if PoolKey(&cfg.Accounts[i]) == s {
			return true
		}
	}
	return false
}

// SplitPoolModel splits an optional pool prefix off a model ID:
// "codebuddy-cn/deepseek-v4.1-flash" (or "<custom prefix>/...") returns
// ("codebuddy-cn", "deepseek-v4.1-flash"). Anything else — a bare ID, or a
// slash ID whose head is not a pool (e.g. "cline-pass/...") — comes back
// untouched with an empty pool.
func SplitPoolModel(model string) (pool, bare string) {
	head, rest, ok := strings.Cut(model, "/")
	if !ok || rest == "" {
		return "", model
	}
	if key := ResolvePoolPrefix(head); key != "" {
		return key, rest
	}
	return "", model
}

// ResolvePoolPrefix maps a custom prefix or a pool key to the pool key; ""
// when s names no pool.
func ResolvePoolPrefix(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	for k, v := range cfg.PoolPrefixes {
		if v == s {
			return k
		}
	}
	if s != "" && isPoolKeyLocked(s) {
		return s
	}
	return ""
}

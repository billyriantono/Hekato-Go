package config

import (
	"encoding/json"
	"strings"
)

// AutoRouteConfig drives the virtual "auto" model: requests are classified into
// a tier, then an (account, model) pair inside that tier is chosen by a bandit.
type AutoRouteConfig struct {
	Enabled bool `json:"enabled"`
	// Sliders 0..1. Quality/Cost shift the tier up/down; Speed and Cost also
	// weigh latency and remaining quota inside the tier.
	QualityWeight float64 `json:"qualityWeight"`
	CostWeight    float64 `json:"costWeight"`
	SpeedWeight   float64 `json:"speedWeight"`
	// Explore is the probability of picking a random candidate (bandit exploration).
	Explore float64 `json:"explore"`
	// Tier model patterns (substring match, case-insensitive, against the
	// model IDs each account advertises). Order = fast, balanced, strong.
	Fast     []string `json:"fast"`
	Balanced []string `json:"balanced"`
	Strong   []string `json:"strong"`
	// Blacklist entries are "provider:pattern" (provider = kiro, codebuddy,
	// grok, codex, clinepass or "*"; pattern = case-insensitive substring of
	// the model ID). A match removes that (provider, model) pair from auto
	// routing only; the same model on another provider stays eligible. A pool
	// key or custom prefix ("codebuddy-cn:", "cn:") narrows it to one pool.
	Blacklist []string `json:"blacklist"`
	// KeywordRules force a tier when a literal keyword appears in the last user
	// message, short-circuiting the heuristic. The operator's escape hatch for
	// work the counting signals cannot see: "kubernetes" is three tokens and a
	// hard problem. First matching rule wins.
	KeywordRules []KeywordRule `json:"keywordRules,omitempty"`
	// AutoThinking lets a plain "auto" request enable thinking by itself when
	// the request classifies as heavy (strong tier). "auto-thinking" always
	// forces it on regardless.
	AutoThinking bool `json:"autoThinking"`
	// Aliases are extra model names that mean "auto", for clients that treat
	// a literal "auto" as their own model switch. Matched case-insensitively.
	Aliases []string `json:"aliases"`
	// A request slower than this marks its (account, model) pair slow: time to
	// first token above SlowTTFTMs, or streaming below SlowMinTPS tokens/s
	// (0 disables either check). A slow pair is skipped for a few minutes
	// while its tier has another candidate.
	SlowTTFTMs int64   `json:"slowTtftMs"`
	SlowMinTPS float64 `json:"slowMinTps"`
}

// IsAlias reports whether model is one of the configured "auto" aliases.
func (c AutoRouteConfig) IsAlias(model string) bool {
	m := strings.TrimSpace(model)
	for _, a := range c.Aliases {
		if a = strings.TrimSpace(a); a != "" && strings.EqualFold(a, m) {
			return true
		}
	}
	return false
}

// KeywordRule maps literal keywords to a tier ("fast", "balanced", "strong").
type KeywordRule struct {
	Keywords []string `json:"keywords"`
	Tier     string   `json:"tier"`
}

// Blacklisted reports whether auto routing must skip model on provider, in
// the account pool pool ("" when the caller has no account).
func (c AutoRouteConfig) Blacklisted(provider AccountProvider, pool, model string) bool {
	m := strings.ToLower(model)
	for _, e := range c.Blacklist {
		prov, pat, ok := strings.Cut(strings.ToLower(strings.TrimSpace(e)), ":")
		if !ok {
			prov, pat = "*", prov
		}
		pat = strings.TrimSpace(pat)
		if pat == "" || (prov != "*" && prov != string(provider) && (pool == "" || ResolvePoolPrefix(prov) != pool)) {
			continue
		}
		if strings.Contains(m, pat) {
			return true
		}
	}
	return false
}

func DefaultAutoRouteConfig() AutoRouteConfig {
	return AutoRouteConfig{
		Enabled:       false,
		QualityWeight: 0.5,
		CostWeight:    0.3,
		SpeedWeight:   0.5,
		Explore:       0.1,
		Fast:          []string{"haiku"},
		Balanced:      []string{"sonnet"},
		Strong:        []string{"opus"},
		AutoThinking:  true,
		Aliases:       []string{"hekato-auto"},
		SlowTTFTMs:    20000,
		SlowMinTPS:    3,
	}
}

func GetAutoRouteConfig() AutoRouteConfig {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	if cfg == nil || cfg.AutoRoute == nil {
		return DefaultAutoRouteConfig()
	}
	return *cfg.AutoRoute
}

func UpdateAutoRouteConfig(c AutoRouteConfig) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfg.AutoRoute = &c
	return Save()
}

func autoRouteToString(c *AutoRouteConfig) string {
	if c == nil {
		return ""
	}
	b, _ := json.Marshal(c)
	return string(b)
}

func autoRouteFromString(s string) *AutoRouteConfig {
	if s == "" {
		return nil
	}
	// Configs saved before these fields existed keep their defaults.
	c := AutoRouteConfig{AutoThinking: true, Aliases: []string{"hekato-auto"}, SlowTTFTMs: 20000, SlowMinTPS: 3}
	if json.Unmarshal([]byte(s), &c) != nil {
		return nil
	}
	return &c
}

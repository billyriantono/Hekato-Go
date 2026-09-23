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
	// routing only; the same model on another provider stays eligible.
	Blacklist []string `json:"blacklist"`
}

// Blacklisted reports whether auto routing must skip model on provider.
func (c AutoRouteConfig) Blacklisted(provider AccountProvider, model string) bool {
	m := strings.ToLower(model)
	for _, e := range c.Blacklist {
		prov, pat, ok := strings.Cut(strings.ToLower(strings.TrimSpace(e)), ":")
		if !ok {
			prov, pat = "*", prov
		}
		pat = strings.TrimSpace(pat)
		if pat == "" || (prov != "*" && prov != string(provider)) {
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
	var c AutoRouteConfig
	if json.Unmarshal([]byte(s), &c) != nil {
		return nil
	}
	return &c
}

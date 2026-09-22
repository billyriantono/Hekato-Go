package config

import "encoding/json"

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

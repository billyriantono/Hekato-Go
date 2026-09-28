package proxy

import (
	"hekato-go/config"
	"testing"
)

// Real request shapes from the production decision ring. The agent toolset is
// present in all of them, which is exactly why it must not decide the tier.
func TestClassifyTierOnRealTraffic(t *testing.T) {
	cases := []struct {
		name string
		sig  routeSignals
		want int
	}{
		{"greeting with the agent toolset", routeSignals{InputTokens: 106, Tools: 13, Turns: 2, Simple: true}, 0},
		{"short question, no tools", routeSignals{InputTokens: 338, Turns: 2}, 0},
		{"mid-size agent turn with thinking", routeSignals{InputTokens: 11136, Tools: 13, Turns: 4, Thinking: true}, 2},
		{"long multimodal agent turn", routeSignals{InputTokens: 69415, Tools: 13, Turns: 48, Images: true, Thinking: true}, 2},
		{"plain code paste", routeSignals{InputTokens: 900, Turns: 1, Code: true}, 0},
		{"refactor request", routeSignals{InputTokens: 900, Turns: 1, Code: true, Reasoning: true}, 1},
		{"big context alone", routeSignals{InputTokens: 50000, Turns: 3}, 2},
		// A one-word reply cannot make 50k of context cheap to process; it only
		// offsets the softer signals around it.
		{"big context, trivial follow-up", routeSignals{InputTokens: 50000, Tools: 13, Turns: 30, Simple: true}, 2},
		{"mid context, trivial follow-up", routeSignals{InputTokens: 13000, Tools: 13, Turns: 30, Simple: true}, 1},
	}
	for _, tc := range cases {
		if got := classifyTier(tc.sig); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, tierNames[got], tierNames[tc.want])
		}
	}
}

// The regression this whole change exists for: identical envelopes, different
// work, must no longer collapse into one tier.
func TestToolCountNoLongerDecidesAlone(t *testing.T) {
	hi := routeSignals{InputTokens: 106, Tools: 13, Turns: 2, Simple: true}
	refactor := routeSignals{InputTokens: 69415, Tools: 13, Turns: 48, Reasoning: true}
	if classifyTier(hi) == classifyTier(refactor) {
		t.Fatal(`"hi" and a 70k-token refactor still classify identically`)
	}
}

func TestAnalyseTextSignals(t *testing.T) {
	cfg := config.AutoRouteConfig{}
	if c := analyseText("hi", cfg); !c.Simple || c.Code || c.Reasoning {
		t.Errorf("greeting: %+v", c)
	}
	if c := analyseText("ok, thanks — that worked and I will now describe at length why", cfg); c.Simple {
		t.Error("a long message that merely starts with 'ok' is not simple")
	}
	if c := analyseText("please refactor this:\n```go\nfunc main() {}\n```", cfg); !c.Code || !c.Reasoning {
		t.Errorf("code + reasoning markers missed: %+v", c)
	}
	if c := analyseText("", cfg); c.Simple || c.Code || c.KeywordTier != -1 {
		t.Errorf("empty text must yield nothing: %+v", c)
	}
}

// An operator keyword rule outranks the heuristic outright.
func TestKeywordRuleOverridesHeuristic(t *testing.T) {
	cfg := config.AutoRouteConfig{KeywordRules: []config.KeywordRule{{Keywords: []string{"kubernetes"}, Tier: "strong"}}}
	c := analyseText("quick question about kubernetes", cfg)
	if c.KeywordTier != 2 {
		t.Fatalf("keyword rule did not match: %+v", c)
	}
	var s routeSignals
	s.KeywordTier = c.KeywordTier + 1
	if got := classifyTier(s); got != 2 {
		t.Errorf("keyword tier ignored: got %s", tierNames[got])
	}
	// An unknown tier name must be ignored rather than silently routed to fast.
	bad := config.AutoRouteConfig{KeywordRules: []config.KeywordRule{{Keywords: []string{"kubernetes"}, Tier: "turbo"}}}
	if c := analyseText("kubernetes", bad); c.KeywordTier != -1 {
		t.Errorf("unknown tier name should not match: %+v", c)
	}
}

// Cheap models must beat dear ones, and quota exhaustion must still bite.
func TestAffordability(t *testing.T) {
	free := routeCandidate{account: &config.Account{}, model: "muse-spark-1.3-contributor-free"}
	dear := routeCandidate{account: &config.Account{}, model: "claude-opus-5"}
	if affordability(free) <= affordability(dear) {
		t.Errorf("free model scored %.3f, opus scored %.3f", affordability(free), affordability(dear))
	}
	drained := routeCandidate{account: &config.Account{UsageLimit: 100, UsagePercent: 0.95}, model: "muse-spark-1.3-contributor-free"}
	if affordability(drained) > 0.1 {
		t.Errorf("a nearly exhausted account must score low, got %.3f", affordability(drained))
	}
	unknown := routeCandidate{account: &config.Account{}, model: "some-unlisted-model"}
	if a := affordability(unknown); a != 0.5 {
		t.Errorf("unpriced model should be treated as mid-market, got %.3f", a)
	}
}

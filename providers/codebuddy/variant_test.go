package codebuddy

import (
	"testing"

	"hekato-go/config"
)

func TestVariantForAccount(t *testing.T) {
	cases := []struct {
		acc  config.Account
		want string
	}{
		{config.Account{AuthMethod: "codebuddy", Provider: "CodeBuddy", Region: "global"}, codeBuddyGlobal.Name},
		{config.Account{AuthMethod: "codebuddy-cn", Provider: "CodeBuddy CN", Region: "cn"}, codeBuddyCN.Name},
		{config.Account{AuthMethod: "codebuddy", Provider: "WorkBuddy", Region: "workbuddy"}, codeBuddyWB.Name},
	}
	for _, c := range cases {
		if got := variantForAccount(&c.acc).Name; got != c.want {
			t.Errorf("%s: variant %q, want %q", c.acc.Provider, got, c.want)
		}
		if kind, err := config.ProviderForAccount(&c.acc); err != nil || kind != config.ProviderCodeBuddy {
			t.Errorf("%s: provider %q (%v), want codebuddy", c.acc.Provider, kind, err)
		}
	}
	// Legacy row with only the free-text provider set still classifies.
	if kind, _ := config.ProviderForAccount(&config.Account{Provider: "WorkBuddy"}); kind != config.ProviderCodeBuddy {
		t.Errorf("legacy WorkBuddy row: provider %q, want codebuddy", kind)
	}
}

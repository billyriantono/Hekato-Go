package pool

import (
	"hekato-go/config"
	"testing"
)

// An operator allowlist must gate routing while leaving the cached catalog
// intact — the panel offers that catalog as the menu to choose from.
func TestEnabledModelsGateRouting(t *testing.T) {
	if err := config.Init(t.TempDir() + "/config.json"); err != nil {
		t.Fatal(err)
	}
	if err := config.AddAccount(config.Account{
		ID: "acc", Email: "a@test", Enabled: true, AccessToken: "t",
		EnabledModels: []string{"gpt-6-astra"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := config.AddAccount(config.Account{
		ID: "open", Email: "b@test", Enabled: true, AccessToken: "t",
	}); err != nil {
		t.Fatal(err)
	}

	p := GetPool()
	p.Reload()
	catalog := []string{"gpt-6-astra", "claude-opus-5", "glm-5.3"}
	p.SetModelList("acc", catalog)
	p.SetModelList("open", catalog)
	t.Cleanup(func() { p.SetModelList("acc", nil); p.SetModelList("open", nil) })

	if got := p.GetForModelByID("acc", "gpt-6-astra", nil); got == nil {
		t.Error("the allowed model must route")
	}
	if got := p.GetForModelByID("acc", "claude-opus-5", nil); got != nil {
		t.Error("a model outside the allowlist must not route")
	}
	// Case and whitespace are the operator's typing, not a routing decision.
	if got := p.GetForModelByID("acc", "  GPT-6-Astra ", nil); got == nil {
		t.Error("matching must be case- and space-insensitive")
	}
	// An account without an allowlist keeps serving everything.
	if got := p.GetForModelByID("open", "claude-opus-5", nil); got == nil {
		t.Error("an empty allowlist must mean everything advertised")
	}
	// The cached catalog is untouched, so the panel can still show all three.
	if list := p.GetModelList("acc"); len(list) != 3 {
		t.Errorf("catalog should stay complete for the picker, got %v", list)
	}
	if got := p.EnabledModels("acc"); len(got) != 1 || got[0] != "gpt-6-astra" {
		t.Errorf("allowlist readback wrong: %v", got)
	}
	if got := p.EnabledModels("open"); got != nil {
		t.Errorf("no allowlist should read back as nil, got %v", got)
	}
}

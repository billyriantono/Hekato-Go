package config

import "testing"

func TestApiKeyAllowsModel(t *testing.T) {
	var none *ApiKeyEntry
	if !none.AllowsModel("anything") {
		t.Fatal("nil key must be unrestricted")
	}
	k := &ApiKeyEntry{AllowedModels: []string{"auto", " Claude-Sonnet-4.5 ", "cline-pass/*"}}
	for _, ok := range []string{"auto", "AUTO", "claude-sonnet-4.5", "cline-pass/glm-5.3"} {
		if !k.AllowsModel(ok) {
			t.Fatalf("%s should be allowed", ok)
		}
	}
	for _, bad := range []string{"claude-opus-4.8", "minimax-m3", ""} {
		if k.AllowsModel(bad) {
			t.Fatalf("%s should be denied", bad)
		}
	}
	if !(&ApiKeyEntry{}).AllowsModel("minimax-m3") {
		t.Fatal("empty allowlist must allow everything")
	}
}

func TestUpdateApiKeyPersistsAllowedModels(t *testing.T) {
	if err := Init(t.TempDir() + "/config.json"); err != nil {
		t.Fatal(err)
	}
	e, err := AddApiKey(ApiKeyEntry{Name: "k", Key: "sk-test-allow", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	e.AllowedModels = []string{"auto"}
	if err := UpdateApiKey(e.ID, e); err != nil {
		t.Fatal(err)
	}
	if got := GetApiKeyEntry(e.ID).AllowedModels; len(got) != 1 || got[0] != "auto" {
		t.Fatalf("allowlist not persisted: %v", got)
	}
	e.AllowedModels = nil
	if err := UpdateApiKey(e.ID, e); err != nil {
		t.Fatal(err)
	}
	if got := GetApiKeyEntry(e.ID).AllowedModels; len(got) != 0 {
		t.Fatalf("clearing allowlist failed: %v", got)
	}
}

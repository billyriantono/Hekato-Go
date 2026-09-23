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

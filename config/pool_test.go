package config

import (
	"path/filepath"
	"testing"
)

func TestPoolKeyAndPrefixes(t *testing.T) {
	resetGlobals(t)
	defer resetGlobals(t)
	t.Setenv(envDBDriver, "sqlite")
	t.Setenv(envDBURL, filepath.Join(t.TempDir(), "kiro.db"))
	if err := Init(filepath.Join(t.TempDir(), "config.json")); err != nil {
		t.Fatalf("Init: %v", err)
	}

	for want, acc := range map[string]Account{
		"codebuddy-cn":    {ProviderKind: "codebuddy", AuthMethod: "codebuddy-cn", Region: "cn"},
		"codebuddy":       {ProviderKind: "codebuddy", AuthMethod: "codebuddy", Region: "global"},
		"workbuddy":       {ProviderKind: "codebuddy", AuthMethod: "codebuddy", Provider: "WorkBuddy", Region: "workbuddy"},
		"kiro":            {ProviderKind: "kiro", Region: "us-east-1"},
		"openrouter.ai":   {ProviderKind: "openai_compat", BaseURL: "https://www.OpenRouter.ai/api/v1"},
		"localhost-11434": {ProviderKind: "anthropic_compat", BaseURL: "http://localhost:11434"},
		"":                {ProviderKind: "openai_compat"},
	} {
		if got := PoolKey(&acc); got != want {
			t.Errorf("PoolKey(%+v) = %q, want %q", acc, got, want)
		}
	}

	for _, bad := range []map[string]string{
		{"codebuddy-cn": "a/b"},                 // slash
		{"codebuddy-cn": "codebuddy"},           // shadows a pool key
		{"codebuddy": "workbuddy"},              // reserves WorkBuddy even before import
		{"codebuddy-cn": "x", "codebuddy": "x"}, // duplicate
		{"nope": "x"},                           // unknown pool
		{"codebuddy-cn": "a:b"},                 // colon (blacklist separator)
	} {
		if UpdatePoolPrefixes(bad) == nil {
			t.Errorf("UpdatePoolPrefixes(%v) accepted", bad)
		}
	}
	if err := AddAccount(Account{ID: "or1", ProviderKind: "openai_compat", BaseURL: "https://openrouter.ai/api/v1", Enabled: true}); err != nil {
		t.Fatalf("AddAccount: %v", err)
	}
	if err := UpdatePoolPrefixes(map[string]string{"codebuddy-cn": "CN", "workbuddy": "wb", "openrouter.ai": "or"}); err != nil {
		t.Fatalf("UpdatePoolPrefixes: %v", err)
	}

	for in, want := range map[string][2]string{
		"cn/deepseek-v4.1-flash":           {"codebuddy-cn", "deepseek-v4.1-flash"},
		"codebuddy-cn/deepseek-v4.1-flash": {"codebuddy-cn", "deepseek-v4.1-flash"},
		"codebuddy/deepseek-v4.1-flash":    {"codebuddy", "deepseek-v4.1-flash"},
		"workbuddy/deepseek-v4.1-flash":    {"workbuddy", "deepseek-v4.1-flash"},
		"wb/deepseek-v4.1-flash":           {"workbuddy", "deepseek-v4.1-flash"},
		"deepseek-v4.1-flash":              {"", "deepseek-v4.1-flash"},
		"cline-pass/foo":                   {"", "cline-pass/foo"},
		"openai/gpt-5":                     {"", "openai/gpt-5"},
		"or/openai/gpt-5":                  {"openrouter.ai", "openai/gpt-5"},
		"openrouter.ai/openai/gpt-5":       {"openrouter.ai", "openai/gpt-5"},
	} {
		if p, b := SplitPoolModel(in); p != want[0] || b != want[1] {
			t.Errorf("SplitPoolModel(%q) = %q, %q; want %q, %q", in, p, b, want[0], want[1])
		}
	}

	// The prefix map must survive a reopen from the SQL store.
	resetGlobals(t)
	if err := Init(filepath.Join(t.TempDir(), "config.json")); err != nil {
		t.Fatalf("re-Init: %v", err)
	}
	if got := GetPoolPrefixes()["codebuddy-cn"]; got != "cn" {
		t.Fatalf("persisted prefix = %q", got)
	}
	if got := GetPoolPrefixes()["workbuddy"]; got != "wb" {
		t.Fatalf("persisted WorkBuddy prefix = %q", got)
	}
}

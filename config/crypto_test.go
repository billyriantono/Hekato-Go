package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecretsSealedAtRest(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "unit-test-key")
	path := filepath.Join(t.TempDir(), "config.json")
	if err := Init(path); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := AddAccount(Account{ID: "a1", Email: "a@x", AccessToken: "ACCESS-SECRET", RefreshToken: "REFRESH-SECRET", Enabled: true}); err != nil {
		t.Fatalf("add account: %v", err)
	}
	if _, err := AddApiKey(ApiKeyEntry{Key: "sk-PLAINKEY", Enabled: true}); err != nil {
		t.Fatalf("add key: %v", err)
	}

	raw, _ := os.ReadFile(path)
	for _, secret := range []string{"ACCESS-SECRET", "REFRESH-SECRET", "sk-PLAINKEY"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("plaintext %q found on disk", secret)
		}
	}
	if !strings.Contains(string(raw), encPrefix) {
		t.Fatal("expected sealed values on disk")
	}
	// In-memory view stays plaintext.
	if GetAccounts()[0].AccessToken != "ACCESS-SECRET" || FindApiKeyByValue("sk-PLAINKEY") == nil {
		t.Fatal("in-memory config must be plaintext")
	}

	// Reload with the same key round-trips.
	if err := Init(path); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if GetAccounts()[0].RefreshToken != "REFRESH-SECRET" {
		t.Fatal("round-trip failed")
	}

	// Reload without the key must fail loudly rather than serve garbage.
	t.Setenv("ENCRYPTION_KEY", "")
	if err := Init(path); err == nil {
		t.Fatal("expected error loading encrypted config without ENCRYPTION_KEY")
	}
}

func TestPlaintextConfigLoadsWithoutKey(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "")
	path := filepath.Join(t.TempDir(), "config.json")
	if err := Init(path); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := AddAccount(Account{ID: "a1", AccessToken: "plain", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), `"plain"`) {
		t.Fatal("without a key, values must stay plaintext")
	}
}

func TestPoolProxyForIsSticky(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "")
	if err := Init(filepath.Join(t.TempDir(), "config.json")); err != nil {
		t.Fatal(err)
	}
	if PoolProxyFor("acc") != "" {
		t.Fatal("empty pool must yield no proxy")
	}
	if err := UpdateProxyPool([]string{"socks5://p1:1080", "socks5://p2:1080", "socks5://p3:1080"}); err != nil {
		t.Fatal(err)
	}
	first := PoolProxyFor("acc")
	for i := 0; i < 10; i++ {
		if PoolProxyFor("acc") != first {
			t.Fatal("assignment must be stable for the same account")
		}
	}
	if !strings.HasPrefix(first, "socks5://p") {
		t.Fatalf("unexpected proxy %q", first)
	}
}

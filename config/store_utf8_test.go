package config

import (
	"path/filepath"
	"testing"
	"unicode/utf8"
)

// A string cut mid-rune (e.g. a byte-truncated upstream error in Chinese)
// must not reach the database as invalid UTF-8: Postgres rejects it, and since
// Save rewrites every row, one bad field blocks every save.
func TestSaveScrubsInvalidUTF8(t *testing.T) {
	resetGlobals(t)
	defer resetGlobals(t)
	t.Setenv(envDBDriver, "sqlite")
	t.Setenv(envDBURL, filepath.Join(t.TempDir(), "kiro.db"))
	if err := Init(filepath.Join(t.TempDir(), "config.json")); err != nil {
		t.Fatalf("Init: %v", err)
	}
	cut := "与你的网络无关。"[:4] // ends inside a 3-byte rune
	if utf8.ValidString(cut) {
		t.Fatal("test string should be invalid UTF-8")
	}
	if err := AddAccount(Account{ID: "u1", AuthMethod: "codebuddy", Enabled: true, WarmupError: cut}); err != nil {
		t.Fatalf("AddAccount: %v", err)
	}

	resetGlobals(t)
	if err := Init(filepath.Join(t.TempDir(), "config.json")); err != nil {
		t.Fatalf("re-Init: %v", err)
	}
	accs := GetAccounts()
	if len(accs) != 1 {
		t.Fatalf("accounts = %d", len(accs))
	}
	if got := accs[0].WarmupError; !utf8.ValidString(got) {
		t.Fatalf("stored warmupError is invalid UTF-8: %q", got)
	}
}

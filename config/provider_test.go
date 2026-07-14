package config

import "testing"

func TestProviderForAccount(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		want    AccountProvider
		wantErr bool
	}{
		{"grok auth method", &Account{AuthMethod: "grok", Provider: "grok"}, ProviderGrok, false},
		{"xai provider", &Account{AuthMethod: "oidc", Provider: "xAI"}, ProviderGrok, false},
		{"codebuddy", &Account{AuthMethod: "codebuddy-cn"}, ProviderCodeBuddy, false},
		{"builder id", &Account{AuthMethod: "idc", Provider: "BuilderId"}, ProviderKiro, false},
		{"social kiro", &Account{AuthMethod: "social", Provider: "Google"}, ProviderKiro, false},
		{"unknown fails closed", &Account{AuthMethod: "oauth", Provider: "new-provider"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProviderForAccount(tt.account)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("got provider=%q err=%v, want provider=%q err=%v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestProviderKindFieldWinsOverLegacyClassification(t *testing.T) {
	// Explicit canonical field beats substring classification: this account's
	// free-text fields would legacy-classify as Kiro ("google" substring).
	acc := &Account{ProviderKind: "grok", AuthMethod: "social", Provider: "Google"}
	got, err := ProviderForAccount(acc)
	if err != nil || got != ProviderGrok {
		t.Fatalf("got provider=%q err=%v, want %q", got, err, ProviderGrok)
	}
}

func TestStampProviderKind(t *testing.T) {
	acc := &Account{AuthMethod: "idc", Provider: "BuilderId"}
	if err := StampProviderKind(acc); err != nil {
		t.Fatal(err)
	}
	if acc.ProviderKind != string(ProviderKiro) {
		t.Fatalf("got providerKind=%q, want %q", acc.ProviderKind, ProviderKiro)
	}
	// Unclassifiable accounts fail closed at creation.
	if err := StampProviderKind(&Account{AuthMethod: "oauth", Provider: "new-provider"}); err == nil {
		t.Fatal("expected error for unclassifiable account")
	}
}

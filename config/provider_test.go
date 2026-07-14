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

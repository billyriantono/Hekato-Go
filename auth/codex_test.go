package auth

import (
	"encoding/base64"
	"testing"
)

// jwtWith constructs an unsigned JWT with the given JSON payload.
// The Codex helper only decodes payload without verifying signatures, so any
// header/signature suffices — pinning the exact claim extraction is the point.
func jwtWith(payloadJSON string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	return header + "." + payload + "."
}

// TestCodexAccountIDFromJWTPrefersAuthNamespace pins the primary claim path
// (https://api.openai.com/auth.chatgpt_account_id) that auth.openai.com issues
// on production Codex tokens. Losing this key would send every request to the
// wrong tenant and trigger the "wrong tenant" 401 that bans accounts.
func TestCodexAccountIDFromJWTPrefersAuthNamespace(t *testing.T) {
	tok := jwtWith(`{"https://api.openai.com/auth":{"chatgpt_account_id":"acct_primary"},"sub":"user_other","account_id":"acct_toplevel"}`)
	if got := CodexAccountIDFromJWT(tok); got != "acct_primary" {
		t.Fatalf("CodexAccountIDFromJWT: got %q, want acct_primary", got)
	}
}

// TestCodexAccountIDFromJWTFallbackKeys covers the alternative claim shapes we
// see across CLI / browser / device-code flows: the top-level chatgpt_account_id,
// then account_id, then sub. Etteum-pool's _extract_account_id fans out the
// same way — matching that keeps 9router-exported tokens accepted verbatim.
func TestCodexAccountIDFromJWTFallbackKeys(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"nested account_id", `{"https://api.openai.com/auth":{"account_id":"acct_nested"}}`, "acct_nested"},
		{"nested user_id", `{"https://api.openai.com/auth":{"user_id":"acct_user"}}`, "acct_user"},
		{"top-level chatgpt_account_id", `{"chatgpt_account_id":"acct_top"}`, "acct_top"},
		{"top-level account_id", `{"account_id":"acct_top2"}`, "acct_top2"},
		{"sub fallback", `{"sub":"acct_sub"}`, "acct_sub"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CodexAccountIDFromJWT(jwtWith(tc.payload)); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCodexAccountIDFromJWTRejectsGarbage guards the trust-on-import path from
// crashing on non-JWT input the operator may paste by accident.
func TestCodexAccountIDFromJWTRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "   ", "not-a-jwt", "only.one", "aaa.bbb.ccc"} {
		if got := CodexAccountIDFromJWT(bad); got != "" {
			t.Fatalf("garbage input %q returned %q", bad, got)
		}
	}
}

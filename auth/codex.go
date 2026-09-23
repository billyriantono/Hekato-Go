package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Codex (OpenAI / ChatGPT) OAuth2 refresh-token flow.
//
// Codex accounts authenticate against auth.openai.com with a public client_id
// (no secret) using the standard OAuth2 refresh_token grant. Reference: etteum-pool
// scripts/auth/app/providers/codex.py and src/proxy/providers/codex.ts.
//
// Only refresh is implemented here: the user obtains the initial token pair
// (access_token + refresh_token) out-of-band and imports it via the admin
// /auth/codex/import route. We don't run the device-code flow on our own — the
// upstream shape varies (browser cookie exchange vs CLI PKCE) and operators
// already have 9router / etteum-pool tooling for it.
const (
	codexTokenURL = "https://auth.openai.com/oauth/token"
	codexClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexScope    = "openid profile email offline_access"
)

// CodexAccountIDFromJWT decodes the unverified access/id token payload and
// returns the chatgpt_account_id claim that the Codex backend requires in the
// `chatgpt-account-id` header. The claim lives at
// "https://api.openai.com/auth.chatgpt_account_id" (auth.openai.com's own
// namespace), with fallbacks to "account_id" / "user_id" inside the same object
// and then to top-level "chatgpt_account_id" / "account_id" / "sub". This
// mirrors etteum-pool's _extract_account_id helper
// (scripts/auth/app/providers/codex.py) so account_id drift on import does
// not push Codex into the "wrong tenant" 401 that bans the account.
//
// No signature verification: this is used only to stamp Account.UserId on
// import and on refresh, never to authenticate. Returns "" if the token is
// not a JWT or carries no usable claim.
func CodexAccountIDFromJWT(token string) string {
	raw := strings.TrimSpace(token)
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	if auth, ok := claims["https://api.openai.com/auth"].(map[string]interface{}); ok {
		for _, key := range []string{"chatgpt_account_id", "account_id", "user_id"} {
			if v, ok := auth[key].(string); ok && v != "" {
				return v
			}
		}
	}
	for _, key := range []string{"chatgpt_account_id", "account_id", "sub"} {
		if v, ok := claims[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// CodexUserInfo is what we read back from the Codex userinfo endpoint after
// refresh or import, used to populate Account.Email / UserId.
type CodexUserInfo struct {
	Email string
	ID    string
}

// RefreshCodexToken refreshes an OpenAI/Codex access token using the
// refresh_token grant against auth.openai.com. The Codex client is a public
// client (no client_secret); refresh tokens are typically long-lived.
//
// Returns: accessToken, refreshToken (kept verbatim if the server omits it),
// expiresIn seconds, error.
func RefreshCodexToken(account *config.Account) (string, string, int, error) {
	if account == nil || strings.TrimSpace(account.RefreshToken) == "" {
		return "", "", 0, fmt.Errorf("codex account missing refresh_token")
	}

	form := url.Values{}
	form.Set("client_id", codexClientID)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", account.RefreshToken)
	form.Set("scope", codexScope)

	req, _ := http.NewRequest(http.MethodPost, codexTokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := GetAuthClientForAccount(account)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("codex refresh failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", "", 0, fmt.Errorf("codex refresh failed: %d %s", resp.StatusCode, string(body))
	}

	var r struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		IDToken      string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", "", 0, fmt.Errorf("parse codex refresh: %w", err)
	}
	if r.AccessToken == "" {
		return "", "", 0, fmt.Errorf("codex refresh returned empty access_token")
	}
	// Refresh-token rotation is optional on auth.openai.com; keep the existing
	// one if the server doesn't issue a new one.
	newRefresh := r.RefreshToken
	if newRefresh == "" {
		newRefresh = account.RefreshToken
	}
	expiresIn := r.ExpiresIn
	if expiresIn == 0 {
		// Default to one hour — matches typical JWT lifetimes issued by
		// auth.openai.com. Better to over-refresh than to serve expired tokens.
		expiresIn = 3600
	}
	// Refresh the chatgpt_account_id claim off whichever JWT we just got —
	// auth.openai.com puts it on the id_token, but on some grants it's only
	// on the access_token. The claim can drift when a user moves workspaces,
	// and a stale value sends Codex to the wrong tenant → 401 → ban. Caller
	// is expected to persist UserId if the value changed.
	if id := CodexAccountIDFromJWT(r.IDToken); id != "" && id != account.UserId {
		account.UserId = id
	} else if id == "" {
		if id := CodexAccountIDFromJWT(r.AccessToken); id != "" && id != account.UserId {
			account.UserId = id
		}
	}
	return r.AccessToken, newRefresh, expiresIn, nil
}

// FetchCodexUserInfo calls the Codex/ChatGPT userinfo endpoint to resolve an
// email and account id from a fresh access token. Both fields are best-effort:
// the import flow tolerates empty strings so a single account can be created
// even when the upstream is rate-limited or returns 401.
func FetchCodexUserInfo(accessToken string) (email, userID string, err error) {
	if strings.TrimSpace(accessToken) == "" {
		return "", "", fmt.Errorf("empty access token")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://api.openai.com/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("codex userinfo failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		// Non-fatal: caller may already have populated email/userID from import.
		return "", "", fmt.Errorf("codex userinfo: %d %s", resp.StatusCode, string(body))
	}

	var r struct {
		Email string `json:"email"`
		ID    string `json:"id"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", "", fmt.Errorf("parse codex userinfo: %w", err)
	}
	return r.Email, r.ID, nil
}

// CodexExpiresAt is the helper to compute the absolute expiry timestamp from
// the relative expires_in returned by the token endpoint. Kept exported so the
// import route can stamp the same value without duplicating arithmetic.
func CodexExpiresAt(expiresIn int) int64 {
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	return time.Now().Unix() + int64(expiresIn)
}

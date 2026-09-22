package auth

import (
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"io"
	"net/http"
	"strings"
	"time"
)

// ClinePass (Cline) OAuth refresh flow.
//
// ClinePass fronts Cline's openai-compatible proxy at api.cline.bot. Accounts
// authenticate with either an API key (clp_…) or a WorkOS JWT issued by the
// Cline browser OAuth flow. The WorkOS JWT must be sent with the literal
// "workos:" prefix; raw JWTs without it are rejected by the upstream.
//
// We only implement refresh here: the user obtains the initial token pair (or
// the API key) out-of-band and imports it via the admin /auth/clinepass/import
// route. Reference: 9router open-sse/providers/registry/clinepass.js +
// open-sse/executors/default.js (refreshCline).
const (
	clinepassBaseURL  = "https://api.cline.bot"
	clinepassTokenURL = clinepassBaseURL + "/api/v1/auth/token"
	clinepassRefresh  = clinepassBaseURL + "/api/v1/auth/refresh"

	clinepassWorkOSPrefix = "workos:"
	// clinepassOAuthClientType is sent on every OAuth token + refresh call
	// (the upstream distinguishes extension from CLI/Web clients).
	clinepassOAuthClientType = "extension"
)

// ClinepassExpiresAt mirrors CodexExpiresAt — converts the relative expires_in
// returned by the token endpoint into an absolute Unix-seconds timestamp.
// Exported so the import route can stamp the same value without duplicating
// arithmetic.
func ClinepassExpiresAt(expiresIn int) int64 {
	if expiresIn <= 0 {
		// WorkOS access tokens issued by ClinePass default to ~1h; default
		// conservatively to 45 minutes so the proxy over-refreshes rather
		// than serve a token whose expiry we never received.
		return time.Now().Add(45 * time.Minute).Unix()
	}
	return time.Now().Unix() + int64(expiresIn)
}

// NormalizeClinepassToken returns the access token in the exact form the
// upstream expects: a raw clp_… key, or a WorkOS JWT prefixed with "workos:".
// Empty / whitespace input is returned unchanged so the import route can use
// it as a validator (empty → invalid).
func NormalizeClinepassToken(raw string) string {
	t := strings.TrimSpace(raw)
	if t == "" {
		return ""
	}
	// API keys go through verbatim.
	if strings.HasPrefix(t, "clp_") {
		return t
	}
	// Already prefixed: keep as-is. ClinePass also accepts the prefix on
	// refresh responses, so this is the canonical form on the wire.
	if strings.HasPrefix(t, clinepassWorkOSPrefix) {
		return t
	}
	// Raw WorkOS JWT (three base64url segments separated by dots): prefix it.
	if looksLikeJWT(t) {
		return clinepassWorkOSPrefix + t
	}
	// Unknown shape — return untouched; the upstream will reject with 401
	// and the operator sees a clear error instead of a silent rewrite.
	return t
}

// IsClinepassAPIKey reports whether the supplied token looks like a ClinePass
// API key (clp_ prefix). Used by the import route to decide between API-key
// and OAuth refresh paths.
func IsClinepassAPIKey(raw string) bool {
	t := strings.TrimSpace(raw)
	return strings.HasPrefix(t, "clp_")
}

// looksLikeJWT is a coarse structural check: three base64url segments joined
// by dots. It does NOT verify the signature — the upstream rejects bad JWTs
// with 401 and the import flow already runs the operator's token against it.
func looksLikeJWT(s string) bool {
	if strings.Count(s, ".") != 2 {
		return false
	}
	for _, seg := range strings.Split(s, ".") {
		if seg == "" {
			return false
		}
	}
	return true
}

// RefreshClinepassToken refreshes a ClinePass OAuth access token against the
// upstream /api/v1/auth/refresh endpoint. Returns the new (possibly workos:-
// prefixed) access token, the rotated refresh token (kept verbatim if the
// server omits one), the relative expires_in seconds, and any error.
//
// API-key accounts have nothing to refresh; callers should check
// IsClinepassAPIKey first and skip the refresh entirely.
func RefreshClinepassToken(account *config.Account) (string, string, int, error) {
	if account == nil {
		return "", "", 0, fmt.Errorf("clinepass account is nil")
	}
	refresh := strings.TrimSpace(account.RefreshToken)
	if refresh == "" {
		return "", "", 0, fmt.Errorf("clinepass account missing refresh_token")
	}

	body := map[string]string{
		"refreshToken": refresh,
		"grantType":    "refresh_token",
		"clientType":   clinepassOAuthClientType,
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, clinepassRefresh, strings.NewReader(string(raw)))
	if err != nil {
		return "", "", 0, fmt.Errorf("build clinepass refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := GetAuthClientForAccount(account).Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("clinepass refresh failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", "", 0, fmt.Errorf("clinepass refresh failed: HTTP %d: %s", resp.StatusCode, truncateForLog(respBody))
	}

	// The upstream wraps responses in {success, data}; unwrap defensively so
	// a future revision that drops the envelope still parses cleanly.
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	parsed := json.RawMessage(respBody)
	if err := json.Unmarshal(respBody, &envelope); err == nil && len(envelope.Data) > 0 {
		parsed = envelope.Data
	}

	var r struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresAt    string `json:"expiresAt"` // ISO-8601 from /refresh
		ExpiresIn    int    `json:"expiresIn"`
	}
	if err := json.Unmarshal(parsed, &r); err != nil {
		return "", "", 0, fmt.Errorf("parse clinepass refresh: %w", err)
	}
	if r.AccessToken == "" {
		return "", "", 0, fmt.Errorf("clinepass refresh returned empty accessToken: %s", truncateForLog(respBody))
	}

	newAccess := NormalizeClinepassToken(r.AccessToken)
	newRefresh := strings.TrimSpace(r.RefreshToken)
	if newRefresh == "" {
		newRefresh = refresh
	}

	expiresIn := r.ExpiresIn
	if expiresIn <= 0 && r.ExpiresAt != "" {
		if t, perr := time.Parse(time.RFC3339, r.ExpiresAt); perr == nil {
			delta := int(time.Until(t).Seconds())
			if delta > 0 {
				expiresIn = delta
			}
		}
	}

	return newAccess, newRefresh, expiresIn, nil
}

// truncateForLog caps the body bytes included in error messages so a giant
// stack trace from the upstream does not blow up the admin log column.
func truncateForLog(b []byte) string {
	const max = 512
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}
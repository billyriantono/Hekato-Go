package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hekato-go/config"
	"io"
	"net/http"
	"strings"
	"time"
)

// CodeBuddy session tokens come from a Keycloak realm (iss like
// https://www.codebuddy.cn/auth/realms/copilot). The console client issues an
// access token (typ Bearer, ~25 days) plus an offline refresh token (typ
// Offline, ~90 days). Renewal goes through CodeBuddy's plugin refresh endpoint;
// the realm's token endpoint rejects the console client (unauthorized_client).

// JWTClaims decodes a JWT payload without verifying the signature. Used only to
// read metadata (iss, azp, exp, typ) from credentials the operator pasted.
func JWTClaims(token string) map[string]interface{} {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil
	}
	var claims map[string]interface{}
	if json.Unmarshal(payload, &claims) != nil {
		return nil
	}
	return claims
}

func claimString(claims map[string]interface{}, key string) string {
	if claims == nil {
		return ""
	}
	s, _ := claims[key].(string)
	return s
}

// CodeBuddyRefreshable reports whether the account carries a Keycloak refresh
// token distinct from its access token (API-key accounts and access-token-only
// imports cannot be refreshed).
func CodeBuddyRefreshable(account *config.Account) bool {
	if account == nil || account.RefreshToken == "" || account.RefreshToken == account.AccessToken {
		return false
	}
	claims := JWTClaims(account.RefreshToken)
	typ := strings.ToLower(claimString(claims, "typ"))
	return claimString(claims, "iss") != "" && (typ == "offline" || typ == "refresh")
}

// RefreshCodeBuddyToken renews the session through CodeBuddy's own refresh
// endpoint (POST /v2/plugin/auth/token/refresh, refresh token in the
// X-Refresh-Token header, as the official CLI does). The realm's Keycloak
// token endpoint rejects the console client directly (unauthorized_client).
// Returns access, refresh, expiresIn.
func RefreshCodeBuddyToken(account *config.Account) (string, string, int, error) {
	if !CodeBuddyRefreshable(account) {
		return "", "", 0, fmt.Errorf("codebuddy: account has no refresh token")
	}
	claims := JWTClaims(account.RefreshToken)
	issuer := strings.ToLower(claimString(claims, "iss"))
	base, domain := "https://www.codebuddy.ai", "www.codebuddy.ai"
	if strings.Contains(issuer, "codebuddy.cn") || strings.Contains(issuer, "copilot.tencent.com") ||
		strings.EqualFold(account.AuthMethod, "codebuddy-cn") {
		base, domain = "https://copilot.tencent.com", "copilot.tencent.com"
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v2/plugin/auth/token/refresh", strings.NewReader("{}"))
	if err != nil {
		return "", "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "CLI/2.106.3 CodeBuddy/2.106.3")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-Domain", domain)
	req.Header.Set("X-Refresh-Token", account.RefreshToken)
	req.Header.Set("X-Auth-Refresh-Source", "plugin")
	req.Header.Set("X-Product", "SaaS")

	resp, err := GetAuthClientForAccount(account).Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("codebuddy refresh: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return "", "", 0, fmt.Errorf("codebuddy refresh: HTTP %d %s", resp.StatusCode, msg)
	}
	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresIn    int    `json:"expiresIn"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.Code != 0 || out.Data.AccessToken == "" {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return "", "", 0, fmt.Errorf("codebuddy refresh: unexpected response %s", msg)
	}
	expiresIn := out.Data.ExpiresIn
	if expiresIn <= 0 {
		if exp := ExpFromAccessTokenJWT(out.Data.AccessToken); exp > 0 {
			expiresIn = int(exp - time.Now().Unix())
		}
	}
	refresh := out.Data.RefreshToken
	if refresh == "" {
		refresh = account.RefreshToken
	}
	return out.Data.AccessToken, refresh, expiresIn, nil
}

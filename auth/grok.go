package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Grok (xAI) OAuth2 device-code flow.
// Public PKCE-less device client — no client secret. Mirrors the official
// grok-cli / grok-build HAR capture and the 9router reference implementation.
const (
	grokTokenURL     = "https://auth.x.ai/oauth2/token"
	grokDeviceCodeURL = "https://auth.x.ai/oauth2/device/code"
	grokClientID     = "b1a00492-073a-47ea-816f-4c329264a828"
	grokScope        = "openid profile email offline_access grok-cli:access api:access conversations:read conversations:write"
	grokReferrer     = "grok-build"
	// ponytail: single global client_id; xAI has no per-tenant registration
)

// GrokSession holds an in-flight device-code login.
type GrokSession struct {
	ID              string
	DeviceCode      string
	UserCode        string
	VerificationURI string
	Interval        int
	ExpiresAt       time.Time
}

var (
	grokSessions = make(map[string]*GrokSession)
	grokMu       sync.RWMutex
)

// StartGrokLogin begins a device-code login and returns the session to poll.
func StartGrokLogin() (*GrokSession, error) {
	form := url.Values{}
	form.Set("client_id", grokClientID)
	form.Set("scope", grokScope)
	form.Set("referrer", grokReferrer)

	req, err := http.NewRequest("POST", grokDeviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device code request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("device code failed: %d %s", resp.StatusCode, string(body))
	}

	var r struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		Interval                int    `json:"interval"`
		ExpiresIn               int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse device code response: %w", err)
	}
	if r.Interval == 0 {
		r.Interval = 5
	}
	if r.ExpiresIn == 0 {
		r.ExpiresIn = 1800
	}

	uri := r.VerificationURIComplete
	if uri == "" {
		uri = r.VerificationURI
	}

	s := &GrokSession{
		ID:              GenerateAccountID(),
		DeviceCode:      r.DeviceCode,
		UserCode:        r.UserCode,
		VerificationURI: uri,
		Interval:        r.Interval,
		ExpiresAt:       time.Now().Add(time.Duration(r.ExpiresIn) * time.Second),
	}

	grokMu.Lock()
	grokSessions[s.ID] = s
	grokMu.Unlock()
	go cleanupExpiredGrokSessions()

	return s, nil
}

// PollGrokAuth polls the token endpoint for the session. status is one of
// "pending", "slow_down", "completed", or "" on terminal error.
func PollGrokAuth(sessionID string) (accessToken, refreshToken string, expiresIn int, status string, err error) {
	grokMu.RLock()
	s, ok := grokSessions[sessionID]
	grokMu.RUnlock()
	if !ok {
		return "", "", 0, "", fmt.Errorf("session not found or expired")
	}
	if time.Now().After(s.ExpiresAt) {
		grokMu.Lock()
		delete(grokSessions, sessionID)
		grokMu.Unlock()
		return "", "", 0, "", fmt.Errorf("device code expired")
	}

	form := url.Values{}
	form.Set("client_id", grokClientID)
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	form.Set("device_code", s.DeviceCode)

	req, _ := http.NewRequest("POST", grokTokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", "", 0, "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 200 {
		var r struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
			IDToken      string `json:"id_token"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return "", "", 0, "", fmt.Errorf("parse token response: %w", err)
		}
		grokMu.Lock()
		delete(grokSessions, sessionID)
		grokMu.Unlock()
		return r.AccessToken, r.RefreshToken, r.ExpiresIn, "completed", nil
	}

	if resp.StatusCode == 400 {
		var r struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &r)
		switch r.Error {
		case "authorization_pending":
			return "", "", 0, "pending", nil
		case "slow_down":
			grokMu.Lock()
			if s2, ok := grokSessions[sessionID]; ok {
				s2.Interval += 5
			}
			grokMu.Unlock()
			return "", "", 0, "slow_down", nil
		case "expired_token":
			grokMu.Lock()
			delete(grokSessions, sessionID)
			grokMu.Unlock()
			return "", "", 0, "", fmt.Errorf("device code expired")
		case "access_denied":
			grokMu.Lock()
			delete(grokSessions, sessionID)
			grokMu.Unlock()
			return "", "", 0, "", fmt.Errorf("user denied authorization")
		default:
			return "", "", 0, "", fmt.Errorf("authorization error: %s", r.Error)
		}
	}

	return "", "", 0, "", fmt.Errorf("unexpected token response: %d %s", resp.StatusCode, string(body))
}

// RefreshGrokToken refreshes an xAI access token using a refresh_token grant.
func RefreshGrokToken(refreshToken string) (string, string, int, error) {
	form := url.Values{}
	form.Set("client_id", grokClientID)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, _ := http.NewRequest("POST", grokTokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("grok refresh failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", 0, fmt.Errorf("grok refresh failed: %d %s", resp.StatusCode, string(body))
	}

	var r struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", "", 0, fmt.Errorf("parse grok refresh: %w", err)
	}
	newRefresh := r.RefreshToken
	if newRefresh == "" {
		newRefresh = refreshToken
	}
	return r.AccessToken, newRefresh, r.ExpiresIn, nil
}

// GetGrokUserInfo decodes the email claim from an xAI id_token. xAI issues no
// userinfo endpoint we rely on; the id_token carries the email claim.
func GetGrokUserInfo(accessToken, idToken string) (email, userID string, err error) {
	if idToken != "" {
		if e := decodeGrokIDTokenEmail(idToken); e != "" {
			return e, "", nil
		}
	}
	// Fallback: call the grok user endpoint.
	req, _ := http.NewRequest("GET", "https://cli-chat-proxy.grok.com/v1/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("grok userinfo failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Email string `json:"email"`
		ID    string `json:"id"`
		Sub   string `json:"sub"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		// Non-fatal: account can still be created without an email.
		return "", "", nil
	}
	if r.Email == "" {
		r.Email = r.ID
	}
	return r.Email, r.ID, nil
}

func decodeGrokIDTokenEmail(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return ""
	}
	payload := parts[1]
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return ""
	}
	var claims struct {
		Email   string `json:"email"`
		Subject string `json:"sub"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}
	if claims.Email != "" {
		return claims.Email
	}
	return claims.Subject
}

func GetGrokSession(sessionID string) *GrokSession {
	grokMu.RLock()
	defer grokMu.RUnlock()
	return grokSessions[sessionID]
}

func cleanupExpiredGrokSessions() {
	grokMu.Lock()
	defer grokMu.Unlock()
	now := time.Now()
	for id, s := range grokSessions {
		if now.After(s.ExpiresAt) {
			delete(grokSessions, id)
		}
	}
}

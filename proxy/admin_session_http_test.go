package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hekato-go/config"
)

// This is the permanent, in-repo version of the admin session acceptance
// suite. It drives the real HTTP surface through Handler.ServeHTTP (not the
// internal handleAdminAPI) so that routing, CORS and the auth gate are all
// exercised the way a browser would hit them.

// adminPreAuthPaths are the only admin endpoints that are reachable without a
// session. Every one of them must be either read-only/one-shot or a
// credential-exchange endpoint.
var adminPreAuthPaths = map[string]bool{
	"/setup/status": true,
	"/setup":        true,
	"/login":        true,
	"/logout":       true,
}

func TestAdminSurfaceRequiresSession(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	srv := httptest.NewServer(h)
	defer srv.Close()

	// Every admin API path that the source mentions must be gated. The list is
	// intentionally broad: a new admin endpoint that forgets the gate shows up
	// here as a 200 instead of a 401.
	paths := []string{
		"/accounts", "/accounts/batch", "/api-keys", "/api-keys/new", "/models",
		"/models/refresh", "/usage", "/usage/logs", "/stats", "/version",
		"/settings", "/settings/general", "/logs", "/system", "/system/one",
		"/providers", "/providers/config", "/proxy-config", "/debug",
		"/routing", "/routing/rules", "/model-prices", "/model-status",
		"/keys", "/config", "/overview", "/docs", "/health", "/cache",
		"/cache/stats", "/cache/clear", "/import", "/export",
	}
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, p := range paths {
		for _, m := range methods {
			url := srv.URL + "/admin/api" + p
			req, err := http.NewRequest(m, url, nil)
			if err != nil {
				t.Fatalf("build request %s %s: %v", m, p, err)
			}
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", m, p, err)
			}
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Errorf("%s %s returned 200 without a session token", m, p)
			}
		}
	}
}

func TestAdminPreAuthEndpointsDoNotLeakData(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	srv := httptest.NewServer(h)
	defer srv.Close()

	// setup/status may be read before setup; it must only say whether the
	// instance is configured, never any secret.
	resp, err := srv.Client().Get(srv.URL + "/admin/api/setup/status")
	if err != nil {
		t.Fatalf("setup/status: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("setup/status = %d", resp.StatusCode)
	}
	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode setup/status: %v", err)
	}
	if len(status) != 1 {
		t.Errorf("setup/status returned unexpected fields: %v", status)
	}
	if _, ok := status["configured"]; !ok {
		t.Errorf("setup/status missing 'configured': %v", status)
	}

	// setup must refuse once the instance is already configured.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/admin/api/setup",
		strings.NewReader(`{"password":"another-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	resp2, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("setup on a configured instance = %d, want 409", resp2.StatusCode)
	}
}

func TestAdminCredentialSmugglingIsRejected(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	srv := httptest.NewServer(h)
	defer srv.Close()

	// None of these may authenticate a request. The raw password in any
	// header or query parameter is not a credential.
	attempts := []struct {
		name   string
		header map[string]string
		url    string
	}{
		{"raw password header", map[string]string{"X-Admin-Password": "admin-secret"}, ""},
		{"raw password in cookie", map[string]string{"Cookie": "hekato_admin_session=admin-secret"}, ""},
		{"raw password as bearer", map[string]string{"Authorization": "Bearer admin-secret"}, ""},
		{"garbage token", map[string]string{"X-Admin-Session": "not-a-real-token"}, ""},
		{"empty token", map[string]string{"X-Admin-Session": ""}, ""},
		{"lowercase header", map[string]string{"x-admin-session": "not-a-real-token"}, ""},
		{"token in query string", nil, "/admin/api/version?token=x&admin_session=x&session=x"},
	}

	for _, a := range attempts {
		url := srv.URL + "/admin/api/version" + a.url
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("%s: %v", a.name, err)
		}
		for k, v := range a.header {
			req.Header.Set(k, v)
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("%s: %v", a.name, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", a.name, resp.StatusCode)
		}
	}
}

func TestAdminSessionRoundTripOverHTTP(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	srv := httptest.NewServer(h)
	defer srv.Close()

	// Login and capture the token plus the cookie.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/admin/api/login",
		strings.NewReader(`{"password":"admin-secret","remember":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login = %d", resp.StatusCode)
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if payload.Token == "" {
		t.Fatal("login returned no token")
	}
	if strings.Contains(payload.Token, "admin-secret") {
		t.Fatal("token contains the password")
	}

	// The session cookie must be HttpOnly so page scripts cannot read it.
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "hekato_admin_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("login did not set a session cookie")
	}
	if !sessionCookie.HttpOnly {
		t.Error("session cookie is not HttpOnly")
	}

	// The token authenticates via the header.
	hdrReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/admin/api/version", nil)
	hdrReq.Header.Set("X-Admin-Session", payload.Token)
	hdrResp, err := srv.Client().Do(hdrReq)
	if err != nil {
		t.Fatalf("header auth: %v", err)
	}
	hdrResp.Body.Close()
	if hdrResp.StatusCode != http.StatusOK {
		t.Errorf("header auth = %d, want 200", hdrResp.StatusCode)
	}

	// The cookie authenticates too.
	ckReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/admin/api/version", nil)
	ckReq.AddCookie(sessionCookie)
	ckResp, err := srv.Client().Do(ckReq)
	if err != nil {
		t.Fatalf("cookie auth: %v", err)
	}
	ckResp.Body.Close()
	if ckResp.StatusCode != http.StatusOK {
		t.Errorf("cookie auth = %d, want 200", ckResp.StatusCode)
	}

	// Logout revokes the session server-side.
	loReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/admin/api/logout", nil)
	loReq.Header.Set("X-Admin-Session", payload.Token)
	loResp, err := srv.Client().Do(loReq)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	loResp.Body.Close()
	if loResp.StatusCode != http.StatusOK {
		t.Errorf("logout = %d, want 200", loResp.StatusCode)
	}

	afterReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/admin/api/version", nil)
	afterReq.Header.Set("X-Admin-Session", payload.Token)
	afterResp, err := srv.Client().Do(afterReq)
	if err != nil {
		t.Fatalf("post-logout request: %v", err)
	}
	afterResp.Body.Close()
	if afterResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("token still valid after logout: status = %d", afterResp.StatusCode)
	}
}

func TestAdminCORSAllowsSessionHeader(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/admin/api/version", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "X-Admin-Session")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	defer resp.Body.Close()

	allowed := resp.Header.Get("Access-Control-Allow-Headers")
	if !strings.Contains(strings.ToLower(allowed), "x-admin-session") {
		t.Errorf("CORS allow-headers = %q, missing X-Admin-Session", allowed)
	}
}

// The remembered vs non-remembered session must differ in cookie lifetime:
// a non-remembered session must be a true session cookie (no Expires/Max-Age)
// so closing the browser discards it.
func TestAdminCookieLifetimeFollowsRememberFlag(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}

	for _, tc := range []struct {
		name        string
		remember    bool
		wantPersist bool
	}{
		{"remembered", true, true},
		{"not remembered", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			body := fmt.Sprintf(`{"password":"admin-secret","remember":%t}`, tc.remember)
			req := httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(body))
			h.handleAdminAPI(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("login = %d, body = %s", rec.Code, rec.Body.String())
			}
			cookies := rec.Result().Cookies()
			var found *http.Cookie
			for _, c := range cookies {
				if c.Name == "hekato_admin_session" {
					found = c
				}
			}
			if found == nil {
				t.Fatal("no session cookie set")
			}
			persistent := !found.Expires.IsZero() || found.MaxAge > 0
			if persistent != tc.wantPersist {
				t.Errorf("persistent = %t, want %t (expires=%v maxAge=%d)",
					persistent, tc.wantPersist, found.Expires, found.MaxAge)
			}
		})
	}
}

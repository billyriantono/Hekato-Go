package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hekato-go/config"
)

func TestAdminLoginIssuesSessionToken(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/login",
		strings.NewReader(`{"password":"admin-secret","remember":true}`))
	h.handleAdminAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if payload.Token == "" {
		t.Fatal("login returned an empty token")
	}
	// The raw password must not appear anywhere in the response.
	if strings.Contains(rec.Body.String(), "admin-secret") {
		t.Fatalf("login response leaked the password: %s", rec.Body.String())
	}
	// The token must authenticate an admin request.
	authed := httptest.NewRecorder()
	authedReq := httptest.NewRequest(http.MethodGet, "/admin/api/version", nil)
	authedReq.Header.Set("X-Admin-Session", payload.Token)
	h.handleAdminAPI(authed, authedReq)
	if authed.Code != http.StatusOK {
		t.Fatalf("authenticated request status = %d, body = %s", authed.Code, authed.Body.String())
	}
}

func TestAdminLoginRejectsWrongPassword(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/login",
		strings.NewReader(`{"password":"wrong"}`))
	h.handleAdminAPI(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("login status = %d, want 401 (body %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "token") {
		t.Fatalf("failed login returned a token: %s", rec.Body.String())
	}
}

func TestAdminRawPasswordHeaderNoLongerAuthenticates(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/version", nil)
	req.Header.Set("X-Admin-Password", "admin-secret")
	h.handleAdminAPI(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: the raw password must not authenticate anymore (body %s)",
			rec.Code, rec.Body.String())
	}
}

func TestAdminLogoutRevokesSession(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	token, _ := h.adminSessions().issue(config.GetPassword(), false)

	// The session works before logout.
	before := httptest.NewRecorder()
	beforeReq := httptest.NewRequest(http.MethodGet, "/admin/api/version", nil)
	beforeReq.Header.Set("X-Admin-Session", token)
	h.handleAdminAPI(before, beforeReq)
	if before.Code != http.StatusOK {
		t.Fatalf("pre-logout status = %d, want 200", before.Code)
	}

	// Logout revokes it.
	logoutRec := httptest.NewRecorder()
	logoutReq := httptest.NewRequest(http.MethodPost, "/admin/api/logout", nil)
	logoutReq.Header.Set("X-Admin-Session", token)
	h.handleAdminAPI(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout status = %d, body = %s", logoutRec.Code, logoutRec.Body.String())
	}

	// The same token is now dead.
	after := httptest.NewRecorder()
	afterReq := httptest.NewRequest(http.MethodGet, "/admin/api/version", nil)
	afterReq.Header.Set("X-Admin-Session", token)
	h.handleAdminAPI(after, afterReq)
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("post-logout status = %d, want 401", after.Code)
	}
}

func TestAdminSessionAcceptedFromCookie(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	token, _ := h.adminSessions().issue(config.GetPassword(), false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/version", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	h.handleAdminAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cookie-auth status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestAdminPasswordChangeRevokesOtherSessions(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	other, _ := h.adminSessions().issue(config.GetPassword(), false)
	mine, _ := h.adminSessions().issue(config.GetPassword(), false)

	// Change the password through the settings endpoint, as the console does.
	req := httptest.NewRequest(http.MethodPost, "/admin/api/settings",
		strings.NewReader(`{"password":"new-secret-123"}`))
	req.Header.Set("X-Admin-Session", mine)
	rec := httptest.NewRecorder()
	h.handleAdminAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("settings status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// The other session is dead.
	otherRec := httptest.NewRecorder()
	otherReq := httptest.NewRequest(http.MethodGet, "/admin/api/version", nil)
	otherReq.Header.Set("X-Admin-Session", other)
	h.handleAdminAPI(otherRec, otherReq)
	if otherRec.Code != http.StatusUnauthorized {
		t.Fatalf("other session status = %d, want 401 after a password change", otherRec.Code)
	}

	// The caller's session was replaced by a fresh token in the response.
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode settings response: %v", err)
	}
	if payload.Token == "" {
		t.Fatal("password change did not return a replacement session token")
	}
	newRec := httptest.NewRecorder()
	newReq := httptest.NewRequest(http.MethodGet, "/admin/api/version", nil)
	newReq.Header.Set("X-Admin-Session", payload.Token)
	h.handleAdminAPI(newRec, newReq)
	if newRec.Code != http.StatusOK {
		t.Fatalf("replacement token status = %d, want 200 (body %s)", newRec.Code, newRec.Body.String())
	}
}

func TestAdminLoginCookieIsSessionCookieWithoutRemember(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/login",
		strings.NewReader(`{"password":"admin-secret","remember":false}`))
	h.handleAdminAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("login did not set the session cookie")
	}
	// Without "remember me" the cookie must not be persisted to disk.
	if !cookie.Expires.IsZero() {
		t.Fatalf("cookie has Expires=%v, want a session cookie with no expiry", cookie.Expires)
	}
	if cookie.MaxAge != 0 {
		t.Fatalf("cookie has MaxAge=%d, want 0 (session cookie)", cookie.MaxAge)
	}
	if !cookie.HttpOnly {
		t.Fatal("cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie SameSite = %v, want Lax", cookie.SameSite)
	}
}

func TestAdminLoginCookiePersistsWithRemember(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/login",
		strings.NewReader(`{"password":"admin-secret","remember":true}`))
	h.handleAdminAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("login did not set the session cookie")
	}
	if cookie.Expires.IsZero() {
		t.Fatal("remembered cookie has no expiry, want a persistent one")
	}
	if !cookie.HttpOnly {
		t.Fatal("cookie is not HttpOnly")
	}
}

func TestAdminLogoutClearsCookie(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	token, _ := h.adminSessions().issue(config.GetPassword(), false)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/logout", nil)
	req.Header.Set("X-Admin-Session", token)
	h.handleAdminAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var found bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			found = true
			if c.Value != "" {
				t.Fatalf("logout cookie still carries a token: %q", c.Value)
			}
			if c.MaxAge >= 0 {
				t.Fatalf("logout cookie MaxAge = %d, want a negative (delete) value", c.MaxAge)
			}
		}
	}
	if !found {
		t.Fatal("logout did not send a cookie-clearing Set-Cookie")
	}
}

func TestSessionStoreExpiry(t *testing.T) {
	s := newSessionStore()
	token, _ := s.issue("pw", false)
	if !s.validate(token, "pw") {
		t.Fatal("freshly issued token did not validate")
	}
	// A different password does not validate.
	if s.validate(token, "other-pw") {
		t.Fatal("token validated against a different password")
	}
	// Revoking kills it.
	s.revoke(token)
	if s.validate(token, "pw") {
		t.Fatal("revoked token still validated")
	}
	// An empty token never validates.
	if s.validate("", "pw") {
		t.Fatal("empty token validated")
	}
}

func TestSessionStoreRevokeAll(t *testing.T) {
	s := newSessionStore()
	a, _ := s.issue("pw", false)
	b, _ := s.issue("pw", false)
	s.revokeAll()
	if s.validate(a, "pw") || s.validate(b, "pw") {
		t.Fatal("tokens survived revokeAll")
	}
}

func TestSessionStorePrunesExpired(t *testing.T) {
	s := newSessionStore()
	token, _ := s.issue("pw", false)
	// Force the session to look expired.
	s.mu.Lock()
	key := sessionKey(token)
	sess := s.sessions[key]
	sess.expires = time.Now().Add(-time.Minute)
	s.sessions[key] = sess
	s.mu.Unlock()
	if s.validate(token, "pw") {
		t.Fatal("expired token validated")
	}
}

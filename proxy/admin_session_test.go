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

// A valid token in a cookie must not authenticate: the browser would attach
// it to forged same-site requests (CSRF). Only the header counts, and no
// endpoint sets a cookie in the first place.
func TestAdminSessionCookieIsNotACredential(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	token, _ := h.adminSessions().issue(config.GetPassword(), false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/version", nil)
	req.AddCookie(&http.Cookie{Name: "hekato_admin_session", Value: token})
	h.handleAdminAPI(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("cookie-auth status = %d, want 401", rec.Code)
	}

	login := httptest.NewRecorder()
	h.handleAdminAPI(login, httptest.NewRequest(http.MethodPost, "/admin/api/login",
		strings.NewReader(`{"password":"admin-secret","remember":true}`)))
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d", login.Code)
	}
	if c := login.Result().Cookies(); len(c) != 0 {
		t.Fatalf("login set cookies %v, want none", c)
	}
}

// A settings save that fails after the password field was sent must not
// change the password, or the console would be left holding a revoked token.
func TestAdminFailedSettingsSaveKeepsPasswordAndSession(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	mine, _ := h.adminSessions().issue(config.GetPassword(), false)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/settings",
		strings.NewReader(`{"password":"new-secret-123","poolPrefixes":{"no-such-pool":"x"}}`))
	req.Header.Set("X-Admin-Session", mine)
	rec := httptest.NewRecorder()
	h.handleAdminAPI(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("settings with invalid poolPrefixes succeeded: %s", rec.Body.String())
	}
	if got := config.GetPassword(); got != "admin-secret" {
		t.Fatalf("password changed to %q despite the failed save", got)
	}
	if !h.adminSessions().validate(mine, config.GetPassword()) {
		t.Fatal("caller's session was revoked by a failed save")
	}
}

// The replacement token after a password change keeps the caller's
// "remember me" lifetime.
func TestAdminPasswordChangeKeepsRememberChoice(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	mine, _ := h.adminSessions().issue(config.GetPassword(), true)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/settings",
		strings.NewReader(`{"password":"new-secret-123"}`))
	req.Header.Set("X-Admin-Session", mine)
	rec := httptest.NewRecorder()
	h.handleAdminAPI(rec, req)
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil || payload.Token == "" {
		t.Fatalf("no replacement token: %d %s", rec.Code, rec.Body.String())
	}
	if !h.adminSessions().remembered(payload.Token) {
		t.Fatal("replacement token lost the remember-me lifetime")
	}
}

func TestAdminLoginIsThrottled(t *testing.T) {
	mustInitConfig(t)
	config.SetPassword("admin-secret")

	h := &Handler{}
	try := func(pw string) int {
		rec := httptest.NewRecorder()
		h.handleAdminAPI(rec, httptest.NewRequest(http.MethodPost, "/admin/api/login",
			strings.NewReader(`{"password":"`+pw+`"}`)))
		return rec.Code
	}
	for i := 0; i < loginAttemptLimit; i++ {
		if code := try("wrong"); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401", i, code)
		}
	}
	if code := try("wrong"); code != http.StatusTooManyRequests {
		t.Fatalf("attempt past the limit = %d, want 429", code)
	}
	// The right password is refused too while the window is used up.
	if code := try("admin-secret"); code != http.StatusTooManyRequests {
		t.Fatalf("correct password past the limit = %d, want 429", code)
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

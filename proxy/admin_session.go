package proxy

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"sync"
	"time"
)

// Admin sessions replace the raw-password cookie/header that the console used
// to carry. The password is exchanged once, at login, for an opaque random
// token. The token is what the browser keeps, so the actual admin password
// never sits in localStorage, sessionStorage, a cookie, or a request header.
//
// Tokens are server-side state: they can be revoked (logout) and they die
// automatically when the admin password changes, because a session is bound
// to the password fingerprint that was current when it was issued.
//
// Storage is in-memory. A restart invalidates every session and the console
// asks for the password again. That is the safe default: a persisted session
// table would be another secret-at-rest surface for a feature that does not
// need to outlive the process.

const (
	// sessionTTL is how long a token lives when the user did not ask to be
	// remembered. Long enough for a working day.
	sessionTTL = 12 * time.Hour
	// rememberTTL is how long a "remember me" token lives.
	rememberTTL = 30 * 24 * time.Hour
	// sessionCookieName is the (optional) cookie mirror of the token header.
	sessionCookieName = "hekato_admin_session"
	// sessionHeaderName is the header the console sends its token in.
	sessionHeaderName = "X-Admin-Session"
)

// adminSession is one issued session. Only the token's hash is kept, so a
// dump of the map does not hand out usable tokens.
type adminSession struct {
	expires time.Time
	// pwHash fingerprints the admin password at issue time. When the password
	// changes, existing sessions stop validating.
	pwHash string
}

// sessionStore holds the live sessions keyed by SHA-256 of the token.
type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]adminSession
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string]adminSession)}
}

// issue mints a new token for the current admin password. The returned token
// is the only copy the caller has; the store keeps just its hash.
func (s *sessionStore) issue(pw string, remember bool) (string, time.Time) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		// crypto/rand never fails on the platforms this runs on, and a
		// predictable token would be worse than a failed login.
		return "", time.Time{}
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	ttl := sessionTTL
	if remember {
		ttl = rememberTTL
	}
	expires := time.Now().Add(ttl)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	s.sessions[sessionKey(token)] = adminSession{
		expires: expires,
		pwHash:  passwordFingerprint(pw),
	}
	return token, expires
}

// validate returns the session for a token, if it exists, is unexpired, and
// was issued against the current admin password.
func (s *sessionStore) validate(token, pw string) bool {
	if token == "" {
		return false
	}
	key := sessionKey(token)

	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[key]
	if !ok {
		return false
	}
	if time.Now().After(sess.expires) {
		delete(s.sessions, key)
		return false
	}
	// A password change invalidates every outstanding session.
	if subtle.ConstantTimeCompare([]byte(sess.pwHash), []byte(passwordFingerprint(pw))) != 1 {
		delete(s.sessions, key)
		return false
	}
	return true
}

// revoke drops a single session (logout). It is a no-op for unknown tokens.
func (s *sessionStore) revoke(token string) {
	if token == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionKey(token))
}

// revokeAll drops every session. Used when the admin password changes.
func (s *sessionStore) revokeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = make(map[string]adminSession)
}

// pruneLocked removes expired sessions. Callers must hold s.mu.
func (s *sessionStore) pruneLocked() {
	now := time.Now()
	for k, v := range s.sessions {
		if now.After(v.expires) {
			delete(s.sessions, k)
		}
	}
}

// sessionKey is the map key for a token: its SHA-256, hex-encoded.
func sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// passwordFingerprint is a stable, non-reversible fingerprint of the admin
// password, used to bind sessions to the password they were issued under.
func passwordFingerprint(pw string) string {
	sum := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(sum[:])
}

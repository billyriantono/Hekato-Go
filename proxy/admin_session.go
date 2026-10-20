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
	// sessionHeaderName is the header the console sends its token in. It is
	// the only place a token is accepted: there is deliberately no cookie,
	// because a cookie the browser attaches on its own makes the admin API
	// forgeable from any same-site page (CSRF).
	sessionHeaderName = "X-Admin-Session"
	// loginAttemptLimit password attempts are allowed per loginAttemptWindow.
	loginAttemptLimit  = 10
	loginAttemptWindow = time.Minute
)

// adminSession is one issued session. Only the token's hash is kept, so a
// dump of the map does not hand out usable tokens.
type adminSession struct {
	expires time.Time
	// pwHash fingerprints the admin password at issue time. When the password
	// changes, existing sessions stop validating.
	pwHash string
	// remember records the "remember me" choice so a replacement token (after
	// a password change) keeps the same lifetime.
	remember bool
}

// sessionStore holds the live sessions keyed by SHA-256 of the token.
type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]adminSession

	// Login throttle: a fixed window counting every password attempt.
	attemptStart time.Time
	attempts     int
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
		expires:  expires,
		pwHash:   passwordFingerprint(pw),
		remember: remember,
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

// remembered reports whether the session for token was issued with
// "remember me". Unknown tokens report false.
func (s *sessionStore) remembered(token string) bool {
	if token == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sessionKey(token)].remember
}

// allowLoginAttempt reserves one password attempt, or reports false when the
// current window is used up. The attempt is counted before the password is
// checked, so concurrent guesses cannot all slip past the limit.
//
// ponytail: one global window, not per client. The client IP is spoofable via
// X-Forwarded-For, and a global cap needs no state per address. The cost is
// that a guesser can keep the real admin out for up to loginAttemptWindow.
func (s *sessionStore) allowLoginAttempt() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.attemptStart) > loginAttemptWindow {
		s.attemptStart, s.attempts = time.Now(), 0
	}
	if s.attempts >= loginAttemptLimit {
		return false
	}
	s.attempts++
	return true
}

// resetLoginAttempts clears the throttle after a successful login.
func (s *sessionStore) resetLoginAttempts() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts = 0
}

// revokeAll drops every session. Used when the admin password changes. The
// fingerprint check in validate is not enough on its own: changing the
// password back to an earlier value would revive that password's sessions.
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

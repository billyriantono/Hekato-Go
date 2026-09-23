package opencodezen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var (
	sessionMu       sync.Mutex
	idMu            sync.Mutex
	activeProjectID string
	activeSessionID string
	sessionExpires  time.Time
	lastMillis      int64
	requestCounter  int64
)

// openCodeSession returns a stable OpenCode CLI project/session pair for the
// normal session lifetime. The request ID remains unique per request.
func openCodeSession() (projectID, sessionID string) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if activeProjectID == "" || activeSessionID == "" || time.Now().After(sessionExpires) {
		activeProjectID = randomHex(20)
		activeSessionID = generateSessionID()
		sessionExpires = time.Now().Add(45 * time.Minute)
	}
	return activeProjectID, activeSessionID
}

// generateSessionID reproduces OpenCode's descending time-plus-counter ID.
func generateSessionID() string {
	return "ses_" + generateOpenCodeID(true)
}

// generateRequestID reproduces OpenCode's ascending time-plus-counter ID.
func generateRequestID() string {
	return "msg_" + generateOpenCodeID(false)
}

func generateOpenCodeID(descending bool) string {
	idMu.Lock()
	now := time.Now().UnixMilli()
	if now != lastMillis {
		lastMillis = now
		requestCounter = 0
	}
	requestCounter++
	counter := requestCounter
	idMu.Unlock()

	value := (now * 0x1000) + counter
	if descending {
		value = ^value
	}
	prefix := fmt.Sprintf("%02x%02x%02x%02x%02x%02x", byte(value>>40), byte(value>>32), byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
	return prefix + randomBase62(14)
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("opencodezen: crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

func randomBase62(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("opencodezen: crypto/rand failed: %v", err))
	}
	out := make([]byte, length)
	for i, v := range b {
		out[i] = base62Chars[int(v)%len(base62Chars)]
	}
	return string(out)
}

package opencodezen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// generateSessionID returns an OpenCode session ID: ses_<12 hex><14 base62>.
func generateSessionID() string {
	return "ses_" + randomHex(6) + randomBase62(14)
}

// generateRequestID returns an OpenCode request ID: msg_<12 hex><14 base62>.
func generateRequestID() string {
	return "msg_" + randomHex(6) + randomBase62(14)
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

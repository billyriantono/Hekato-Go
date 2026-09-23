package proxy

import (
	"testing"
	"time"
)

func TestAccountFailureClassifiers(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) bool
		msg  string
	}{
		{name: "quota", fn: isQuotaErrorMessage, msg: "HTTP 429: quota exhausted"},
		{name: "overage", fn: isOverageErrorMessage, msg: "HTTP 402 from Kiro IDE: OVERAGE limit exceeded"},
		{name: "suspension", fn: isSuspensionErrorMessage, msg: "Your User ID temporarily is suspended"},
		{name: "profile", fn: isProfileUnavailableErrorMessage, msg: "no available Kiro profile"},
		{name: "auth", fn: isAuthErrorMessage, msg: "Authentication failed - token invalid or expired"},
	}

	for _, tc := range tests {
		if !tc.fn(tc.msg) {
			t.Fatalf("%s classifier did not match %q", tc.name, tc.msg)
		}
	}
}

func TestCapResetFromMessage(t *testing.T) {
	until, ok := capResetFromMessage(`Error 429: You have reached your monthly Clinepass limit. The limit resets in 13d 9h, please try again later.`)
	if !ok {
		t.Fatal("expected a reset time")
	}
	if d := time.Until(until); d < 13*24*time.Hour+9*time.Hour-time.Minute || d > 13*24*time.Hour+9*time.Hour+2*time.Minute {
		t.Fatalf("unexpected duration %v", d)
	}
	if _, ok := capResetFromMessage("quota exhausted"); ok {
		t.Fatal("no hint must not produce a time")
	}
}

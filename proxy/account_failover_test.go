package proxy

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"hekato-go/config"
	"hekato-go/providers"
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

func TestWarmupRateLimited(t *testing.T) {
	codebuddy := &config.Account{ProviderKind: "codebuddy"}
	codebuddyCN := &config.Account{AuthMethod: "codebuddy-cn"} // legacy row, classified by text
	kiro := &config.Account{ProviderKind: "kiro"}
	limited := providers.Errorf(429, "HTTP 429 from codebuddy: too many requests")
	cases := []struct {
		name    string
		account *config.Account
		err     error
		want    bool
	}{
		{"codebuddy global 429", codebuddy, limited, true},
		{"codebuddy china 429", codebuddyCN, limited, true},
		{"wrapped 429", codebuddy, fmt.Errorf("usage: %w", limited), true},
		{"other status", codebuddy, providers.Errorf(500, "HTTP 500"), false},
		{"429 only in text", codebuddy, errors.New("request id req-429-abc failed"), false},
		{"other provider 429", kiro, limited, false},
	}
	for _, c := range cases {
		if got := warmupRateLimited(c.account, c.err); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSetEnabledByOperator(t *testing.T) {
	a := config.Account{Enabled: false, BanStatus: banStatusRateLimited, BanReason: "Auto-disabled: 429", BanTime: 1}
	setEnabledByOperator(&a, true)
	if !a.Enabled || a.BanStatus != "ACTIVE" || a.BanReason != "" || a.BanTime != 0 {
		t.Fatalf("enable left ban state behind: %+v", a)
	}
	a = config.Account{Enabled: false, BanStatus: "BANNED", BanReason: "Authentication failed", BanTime: 1}
	setEnabledByOperator(&a, false)
	if a.BanReason != "" || autoBanned(&a) {
		t.Fatalf("manual disable still looks auto-banned, auto-recover would undo it: %+v", a)
	}
}

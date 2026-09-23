package commandcode

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"hekato-go/config"
)

func TestFetchUsageMapsCreditsAndWindows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer user_k" {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/alpha/whoami":
			w.Write([]byte(`{"success":true,"user":{"id":"u1","email":"a@b"},"org":null}`))
		case "/alpha/billing/credits":
			w.Write([]byte(`{"credits":{"monthlyCredits":7.5,"purchasedCredits":0,"freeCredits":0},"windowLimits":{"fiveHour":{"used":0.5,"cap":3,"resetAt":1790169595364},"weekly":{"used":1,"cap":6,"resetAt":1}}}`))
		case "/alpha/billing/subscriptions":
			w.Write([]byte(`{"success":true,"data":{"planId":"individual-go","status":"active","currentPeriodEnd":"2026-10-08T05:11:18.000Z"}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	usageBase = srv.URL
	info, err := FetchUsage(&config.Account{ID: "x", AccessToken: "user_k"})
	if err != nil {
		t.Fatal(err)
	}
	if info.Email != "a@b" || info.SubscriptionTitle != "Command Code Go" || info.UsageLimit != 10 || info.UsageCurrent != 2.5 || info.UsagePercent != 25 {
		t.Fatalf("plan mapping wrong: %+v", info)
	}
	if info.TrialUsageCurrent != 0.5 || info.TrialUsageLimit != 3 || info.TrialExpiresAt != 1790169595 || info.NextResetDate != "2026-10-08" {
		t.Fatalf("window mapping wrong: %+v", info)
	}
}

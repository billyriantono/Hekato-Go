package opencodego

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"hekato-go/config"
)

func TestFetchUsageMapsWindows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"usage":{"rolling":{"status":"ok","percent":12.5,"resetsAt":"2026-09-23T12:00:00.000Z"},"weekly":{"status":"rate-limited","percent":100,"resetsAt":"2026-09-30T00:00:00.000Z"},"monthly":{"status":"ok","percent":40,"resetsAt":"2026-10-23T00:00:00.000Z"}}}`))
	}))
	defer srv.Close()
	usageURL = srv.URL
	info, err := FetchUsage(&config.Account{ID: "x", AccessToken: "oc_go"})
	if err != nil {
		t.Fatal(err)
	}
	if info.UsagePercent != 12.5 || info.UsageLimit != 100 || info.TrialUsagePercent != 100 || info.TrialStatus != "WEEKLY RATE-LIMITED" || info.NextResetDate == "" {
		t.Fatalf("mapping wrong: %+v", info)
	}
	usageURL = goUsageURL
	if _, err := FetchUsage(&config.Account{}); err == nil {
		t.Fatal("missing key must error")
	}
}

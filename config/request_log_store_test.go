package config

import (
	"path/filepath"
	"testing"
)

// TestRequestLogsJSONBackend verifies that the jsonStore end-to-end:
// writes go through, LoadRecent returns ascending-time entries, and Clear wipes them.
func TestRequestLogsJSONBackend(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	resetGlobals(t)
	t.Cleanup(func() { resetGlobals(t) })
	if err := Init(cfgPath); err != nil {
		t.Fatalf("init: %v", err)
	}
	rs := RequestLogs()
	if rs == nil {
		t.Fatal("expected RequestLogs() to be non-nil for json backend")
	}

	// Three entries, newest last.
	in := []PersistedRequestLog{
		{Time: 1000, Endpoint: "openai", Model: "gpt-4", AccountID: "a", Status: "success", Tokens: 10, Credits: 0.1, Duration: 200},
		{Time: 1001, Endpoint: "claude", Model: "claude-3", AccountID: "b", Status: "error", Error: "boom", ErrorType: "unknown"},
		{Time: 1002, Endpoint: "openai", Model: "gpt-4", AccountID: "a", Status: "success", Tokens: 5, Credits: 0.05, Duration: 100},
	}
	if err := rs.Append(in); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, err := rs.LoadRecent(500)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 entries, got %d", len(got))
	}
	for i := range got {
		if got[i].Time != in[i].Time {
			t.Fatalf("entry %d time mismatch: got %d want %d", i, got[i].Time, in[i].Time)
		}
	}

	// Dedup by Time: re-appending the same batch should not grow.
	if err := rs.Append(in); err != nil {
		t.Fatalf("re-append: %v", err)
	}
	got, err = rs.LoadRecent(500)
	if err != nil {
		t.Fatalf("load2: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("after dedupe want 3 entries, got %d", len(got))
	}

	// LoadRecent honours limit.
	got, err = rs.LoadRecent(2)
	if err != nil {
		t.Fatalf("load-limited: %v", err)
	}
	if len(got) != 2 || got[0].Time != 1001 || got[1].Time != 1002 {
		t.Fatalf("limit=2 expected [1001, 1002], got %+v", got)
	}

	// Clear empties.
	if err := rs.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	got, err = rs.LoadRecent(500)
	if err != nil {
		t.Fatalf("load-after-clear: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("after clear want 0 entries, got %d", len(got))
	}
}

// TestRequestLogsSQLBackend verifies the sqlStore end-to-end through the public
// RequestLogStore methods. SQLite is the test surrogate for both SQL drivers
// (the code is shared via the same sqlStore type).
func TestRequestLogsSQLBackend(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	resetGlobals(t)
	t.Cleanup(func() { resetGlobals(t) })
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", filepath.Join(dir, "hekato.db"))
	if err := Init(cfgPath); err != nil {
		t.Fatalf("init: %v", err)
	}
	rs := RequestLogs()
	if rs == nil {
		t.Fatal("expected RequestLogs() to be non-nil for sqlite backend")
	}

	in := []PersistedRequestLog{
		{Time: 2000, Endpoint: "responses", Model: "gpt-5", Status: "success", Tokens: 42, Duration: 500},
		{Time: 2001, Endpoint: "responses", Model: "gpt-5", Status: "error", Error: "rate limited", ErrorType: "quota"},
	}
	if err := rs.Append(in); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, err := rs.LoadRecent(100)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 2 || got[0].Time != 2000 || got[1].Time != 2001 {
		t.Fatalf("unexpected: %+v", got)
	}
	if got[1].Error != "rate limited" || got[1].ErrorType != "quota" {
		t.Fatalf("error fields not preserved: %+v", got[1])
	}
}

package proxy

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestPerfTrackerMeasuresTTFTAndTPS asserts the streaming perf tracker emits a
// non-zero time-to-first-byte and a TPS estimate derived from the running
// token counter. We sleep between markFirstByte and finalise so the millisecond
// rounding produces a stable, observable TTFT.
func TestPerfTrackerMeasuresTTFTAndTPS(t *testing.T) {
	start := time.Now().Add(-200 * time.Millisecond)
	p := newPerfTracker(start)
	p.markFirstByte()
	p.addTokens("hello world, this is a streamed response chunk")
	p.setFinalTokens(64)

	perf := p.finalise()
	if perf.ttftMs <= 0 {
		t.Fatalf("expected positive ttft, got %d", perf.ttftMs)
	}
	if perf.tps <= 0 {
		t.Fatalf("expected positive tps, got %v", perf.tps)
	}
	// Authoritative token count wins over the rune estimate. finalise rounds
	// TPS to 2 decimals (display precision), so compare against the rounded
	// expected — 320.00 ≠ 319.999... but should equal 320.00.
	want := float64(64) / time.Since(start).Seconds()
	want = math.Round(want*100) / 100
	if math.Abs(perf.tps-want) > 0.01 {
		t.Fatalf("tps = %v, want %v", perf.tps, want)
	}
}

// TestPerfTrackerNilSafe lets the helper satisfy no-stream / cache-hit paths
// that may receive a nil tracker through an interface-style call.
func TestPerfTrackerNilSafe(t *testing.T) {
	var p *perfTracker
	p.markFirstByte()
	p.addTokens("ignored")
	p.setFinalTokens(10)
	perf := p.finalise()
	if perf != (requestPerf{}) {
		t.Fatalf("nil tracker should yield zero perf, got %+v", perf)
	}
}

// TestBindClientLogMetaPrefersForwardedFor asserts the leftmost X-Forwarded-For
// entry is what the dashboard sees, falling back to X-Real-IP and finally
// RemoteAddr. Truncation kicks in for overlong UAs.
func TestBindClientLogMetaPrefersForwardedFor(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	r.Header.Set("User-Agent", "claude-cli/1.0")
	r.Header.Set("X-Forwarded-For", "203.0.113.42, 10.0.0.1")
	r.RemoteAddr = "127.0.0.1:54321"

	ua, ip := clientLogMeta(withClientLogMeta(httptest.NewRecorder(), r))
	if ua != "claude-cli/1.0" {
		t.Fatalf("ua = %q, want claude-cli/1.0", ua)
	}
	if ip != "203.0.113.42" {
		t.Fatalf("ip = %q, want 203.0.113.42", ip)
	}
}

// TestBindClientLogMetaFallsBackToRealIP checks the X-Real-IP branch when
// X-Forwarded-For is absent.
func TestBindClientLogMetaFallsBackToRealIP(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	r.Header.Set("X-Real-IP", "198.51.100.7")
	r.RemoteAddr = "127.0.0.1:54321"

	_, ip := clientLogMeta(withClientLogMeta(httptest.NewRecorder(), r))
	if ip != "198.51.100.7" {
		t.Fatalf("ip = %q, want 198.51.100.7", ip)
	}
}

// TestBindClientLogMetaTruncatesLongUA guards against a malicious UA blowing
// up the in-memory ring and dashboard column width.
func TestBindClientLogMetaTruncatesLongUA(t *testing.T) {
	long := strings.Repeat("x", logUserAgentMaxLen+50)
	r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	r.Header.Set("User-Agent", long)

	ua, _ := clientLogMeta(withClientLogMeta(httptest.NewRecorder(), r))
	// The cap is a byte ceiling (the dashboard column renders by visual width,
	// not rune count); `…` is 3 bytes, so a 199-byte head + 3-byte ellipsis
	// lands at 202 bytes.
	if len(ua) > logUserAgentMaxLen+2 {
		t.Fatalf("len(ua) = %d, exceeds cap+ellipsis", len(ua))
	}
	if !strings.HasSuffix(ua, "…") {
		t.Fatalf("truncated ua should end with ellipsis, got %q", ua[len(ua)-3:])
	}
}

// TestNewLogEntryStampsClientMetadata ensures every request log entry produced
// through a metadata-carrying writer picks up that request's UA + IP; the
// metadata rides on the writer, so concurrent requests never see each other's.
func TestNewLogEntryStampsClientMetadata(t *testing.T) {
	h := &Handler{}
	r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	r.Header.Set("User-Agent", "openai-python/1.50.0")
	r.Header.Set("X-Forwarded-For", "198.51.100.7")

	w := withClientLogMeta(httptest.NewRecorder(), r)

	entry := h.newLogEntry(w, "openai", "gpt-5", "codex-acc-1", "success", 100, 64, 0.30, 1820, 245, 47.6)
	if entry.UserAgent != "openai-python/1.50.0" {
		t.Fatalf("ua = %q, want openai-python/1.50.0", entry.UserAgent)
	}
	if entry.ClientIP != "198.51.100.7" {
		t.Fatalf("ip = %q, want 198.51.100.7", entry.ClientIP)
	}
	if entry.OutputTokens != 64 || entry.TTFTMs != 245 || entry.TPS != 47.6 || entry.Duration != 1820 {
		t.Fatalf("perf fields not threaded through newLogEntry: %+v", entry)
	}
}

// TestClientLogMetaIsolatedPerRequest is the regression for the shared-field
// binding: two in-flight requests must each log their own UA / IP.
func TestClientLogMetaIsolatedPerRequest(t *testing.T) {
	h := &Handler{}
	r1 := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	r1.Header.Set("User-Agent", "client-a")
	r1.Header.Set("X-Forwarded-For", "10.0.0.1")
	r2 := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	r2.Header.Set("User-Agent", "client-b")
	r2.Header.Set("X-Forwarded-For", "10.0.0.2")
	w1 := withClientLogMeta(httptest.NewRecorder(), r1)
	w2 := withClientLogMeta(httptest.NewRecorder(), r2)

	e1 := h.newLogEntry(w1, "claude", "m", "a", "success", 1, 1, 0, 1, 0, 0)
	e2 := h.newLogEntry(w2, "claude", "m", "a", "success", 1, 1, 0, 1, 0, 0)
	if e1.UserAgent != "client-a" || e1.ClientIP != "10.0.0.1" || e2.UserAgent != "client-b" || e2.ClientIP != "10.0.0.2" {
		t.Fatalf("metadata leaked between requests: %+v %+v", e1, e2)
	}
	if _, ok := w1.(http.Flusher); !ok {
		t.Fatal("wrapped writer must still expose http.Flusher for SSE handlers")
	}
}

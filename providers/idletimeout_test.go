package providers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A stream that keeps trickling bytes must survive well past the idle window,
// while one that goes silent must be cut.
func TestIdleTimeoutTransport(t *testing.T) {
	stall := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := w.(http.Flusher)
		if r.URL.Path == "/stall" {
			w.Write([]byte("x"))
			f.Flush()
			<-stall // never another byte
			return
		}
		for i := 0; i < 6; i++ { // 6 * 40ms = 240ms, far past the 100ms idle window
			w.Write([]byte("y"))
			f.Flush()
			time.Sleep(40 * time.Millisecond)
		}
	}))
	defer srv.Close()
	defer close(stall)

	client := &http.Client{Transport: withIdleTimeout(http.DefaultTransport, 100*time.Millisecond)}

	resp, err := client.Get(srv.URL + "/trickle")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("trickling stream was cut: %v (read %q)", err, body)
	}
	if len(body) != 6 {
		t.Fatalf("trickling stream lost data: %q", body)
	}

	resp, err = client.Get(srv.URL + "/stall")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err == nil {
		t.Fatal("stalled stream should have been cancelled")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("stalled stream took %s to fail", elapsed)
	}
}

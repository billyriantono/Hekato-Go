package providers

import (
	"context"
	"io"
	"net/http"
	"time"
)

// streamIdleTimeout is how long a streaming response may go without producing
// a single byte before it is treated as dead. Long reasoning turns are fine —
// they keep emitting — but an upstream that accepts the request and then goes
// silent is not, and used to sit there until the old 5-minute overall timeout
// cut it (or, with no overall timeout, forever).
const streamIdleTimeout = 2 * time.Minute

// withIdleTimeout wraps next so every response body it returns must keep
// producing bytes. idle <= 0 returns next untouched.
func withIdleTimeout(next http.RoundTripper, idle time.Duration) http.RoundTripper {
	if idle <= 0 || next == nil {
		return next
	}
	return &idleTimeoutTransport{next: next, idle: idle}
}

type idleTimeoutTransport struct {
	next http.RoundTripper
	idle time.Duration
}

func (t *idleTimeoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// A cancellable child context is the only handle that reaches into an
	// in-flight body read; firing it is what turns a silent stream into an error.
	ctx, cancel := context.WithCancel(req.Context())
	resp, err := t.next.RoundTrip(req.WithContext(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	if resp.Body == nil {
		cancel()
		return resp, nil
	}
	b := &idleBody{rc: resp.Body, idle: t.idle, cancel: cancel}
	b.timer = time.AfterFunc(t.idle, cancel)
	resp.Body = b
	return resp, nil
}

// idleBody cancels the request when no byte arrives for idle.
type idleBody struct {
	rc     io.ReadCloser
	timer  *time.Timer
	idle   time.Duration
	cancel context.CancelFunc
}

func (b *idleBody) Read(p []byte) (int, error) {
	n, err := b.rc.Read(p)
	if n > 0 {
		b.timer.Reset(b.idle)
	}
	if err != nil {
		b.timer.Stop()
	}
	return n, err
}

func (b *idleBody) Close() error {
	b.timer.Stop()
	err := b.rc.Close()
	b.cancel() // always release the context, even on a clean close
	return err
}

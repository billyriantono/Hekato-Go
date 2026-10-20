package proxy

import (
	"net/http/httptest"
	"testing"
)

func TestPinModelPoolRewritesAutoAlias(t *testing.T) {
	w := &logMetaWriter{ResponseWriter: httptest.NewRecorder()}
	for _, in := range []string{"hekato-auto", "HEKATO-AUTO", " hekato-auto "} {
		if m, ok := pinModelPool(w, "", in); !ok || m != autoModelName {
			t.Fatalf("%q → %q, %v; want auto", in, m, ok)
		}
	}
	if m, _ := pinModelPool(w, "", "claude-sonnet-4.6"); m != "claude-sonnet-4.6" {
		t.Fatalf("non-alias rewritten to %q", m)
	}
}

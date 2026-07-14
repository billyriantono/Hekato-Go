package auth

import "testing"

func TestDecodeGrokIDTokenEmail(t *testing.T) {
	// header.payload.signature ; payload = base64url({"email":"a@x.ai","sub":"u1"})
	payload := "eyJlbWFpbCI6ImFAeC5haSIsInN1YiI6InUxIn0"
	id := "x." + payload + ".y"
	if got := decodeGrokIDTokenEmail(id); got != "a@x.ai" {
		t.Fatalf("email = %q, want a@x.ai", got)
	}
	if got := decodeGrokIDTokenEmail("not.a.jwt"); got != "" {
		t.Fatalf("bad token should yield empty, got %q", got)
	}
}

package proxy

import "testing"

func TestMaskIP(t *testing.T) {
	cases := map[string]string{
		"203.0.113.7":      "203.0.x.x",
		"::ffff:10.1.2.3":  "10.1.x.x",
		"2001:db8:1::5":    "2001:db8:…",
		"":                 "",
		"not-an-ip":        "",
		"2001:db8::1%eth0": "2001:db8:…",
	}
	for in, want := range cases {
		if got := maskIP(in); got != want {
			t.Errorf("maskIP(%q) = %q, want %q", in, got, want)
		}
	}
}

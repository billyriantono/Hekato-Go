package proxy

import "testing"

func TestVersionNewer(t *testing.T) {
	for _, tc := range []struct {
		release, running string
		want             bool
	}{
		{"1.1.4", "1.1.3", true},
		{"1.2.0", "1.1.9", true},
		{"2.0.0", "1.10.0", true},
		{"1.10.0", "1.9.0", true},
		{"1.1.3", "1.1.3", false},
		{"1.1.3", "1.1.3-4-gabc1234", false}, // built past the tag
		{"1.1.3", "v1.1.4", false},
		{"1.1.4", "dev-8b31efa-dirty", false},
		{"1.1.4", "8b31efa", false},
	} {
		if got := versionNewer(tc.release, tc.running); got != tc.want {
			t.Errorf("versionNewer(%q, %q) = %v, want %v", tc.release, tc.running, got, tc.want)
		}
	}
}

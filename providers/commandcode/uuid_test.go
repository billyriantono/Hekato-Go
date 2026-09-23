package commandcode

import (
	"regexp"
	"testing"
)

func TestNewUUIDIsValidV4(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for i := 0; i < 50; i++ {
		id := newUUID()
		if !re.MatchString(id) {
			t.Fatalf("bad uuid %q", id)
		}
	}
}

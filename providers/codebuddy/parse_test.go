package codebuddy

import "testing"

func TestParseCodeBuddyCredentials(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjE3OTIyOTQ3Mjh9.sig"
	obj := `{"access_token":"` + jwt + `","uid":"u1","refresh_token":"eyJhbGciOiJIUzUxMiJ9.eyJ0eXAiOiJPZmZsaW5lIn0.x"}`
	cases := map[string]int{
		"sk-key\n" + jwt:                2,
		obj:                             1,
		"[" + obj + "," + obj + "]":     2,
		obj + "\n" + obj + "\nsk-other": 3,
		"{\n  \"access_token\": \"" + jwt + "\",\n  \"uid\": \"u2\"\n}": 1,
	}
	for in, want := range cases {
		got := parseCodeBuddyCredentials(in)
		if len(got) != want {
			t.Fatalf("%q: want %d creds, got %d", in[:20], want, len(got))
		}
	}
	c := parseCodeBuddyCredentials(obj)[0]
	if c.Access != jwt || c.UID != "u1" || c.Refresh == "" {
		t.Fatalf("fields not parsed: %+v", c)
	}
}

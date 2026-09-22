package codebuddy

import "testing"

func TestParseRealTokenJSON(t *testing.T) {
	blob := "{\n  \"access_token\": \"eyJhbGciOiJSUzI1NiJ9.eyJleHAiOjE3OTIyOTQzNTQsInR5cCI6IkJlYXJlciIsImF6cCI6ImNvbnNvbGUiLCJpc3MiOiJodHRwczovL3d3dy5jb2RlYnVkZHkuY24vYXV0aC9yZWFsbXMvY29waWxvdCJ9.sig\",\n  \"uid\": \"bac88f6a-5feb-4e44-ab8e-a9c5eef9f975\",\n  \"refresh_token\": \"eyJhbGciOiJIUzUxMiJ9.eyJ0eXAiOiJPZmZsaW5lIiwiaXNzIjoiaHR0cHM6Ly93d3cuY29kZWJ1ZGR5LmNuL2F1dGgvcmVhbG1zL2NvcGlsb3QifQ.sig\"\n}"
	got := parseCodeBuddyCredentials(blob)
	if len(got) != 1 {
		t.Fatalf("want 1 cred, got %d: %+v", len(got), got)
	}
	if got[0].UID == "" || got[0].Refresh == "" {
		t.Fatalf("uid/refresh missing: %+v", got[0])
	}
}

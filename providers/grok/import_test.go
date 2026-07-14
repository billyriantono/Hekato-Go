package grok

import "testing"

const sample = `[
  {"email":"a@x.ai","tokens":{"access_token":"at1","refresh_token":"rt1","expires_at":"2026-07-14T08:18:50.098758Z","expires_in":21600,"email":"a@x.ai"}},
  {"email":"b@x.ai","tokens":{"access_token":"at2","refresh_token":"rt2","expires_at":"2026-07-14T08:19:02.111687Z","expires_in":21600,"email":"b@x.ai"}}
]`

const ndjson = `{"email":"d@x.ai","tokens":{"access_token":"at4"}}
{"email":"e@x.ai","tokens":{"access_token":"at5"}}`

func TestParseGrokImportEntries(t *testing.T) {
	arr, err := parseGrokImportEntries([]byte(sample))
	if err != nil || len(arr) != 2 {
		t.Fatalf("array: got %d err %v", len(arr), err)
	}
	if arr[0].Tokens.AccessToken != "at1" {
		t.Fatalf("arr[0] access = %q", arr[0].Tokens.AccessToken)
	}

	one, err := parseGrokImportEntries([]byte(`{"email":"c@x.ai","tokens":{"access_token":"at3"}}`))
	if err != nil || len(one) != 1 || one[0].Tokens.AccessToken != "at3" {
		t.Fatalf("single: %+v err %v", one, err)
	}

	ndj, err := parseGrokImportEntries([]byte(ndjson))
	if err != nil || len(ndj) != 2 {
		t.Fatalf("ndjson: got %d err %v", len(ndj), err)
	}

	if _, err := parseGrokImportEntries([]byte("   ")); err == nil {
		t.Fatal("expected error on empty body")
	}
}

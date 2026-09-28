package providers

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Inference must never run on the 30-second REST client: a reasoning model can
// legitimately take minutes, and the cap surfaces as "context deadline
// exceeded ... while reading body" long after the request looked healthy.
// This guards the rule at the only place it can be enforced cheaply — the
// source — because the failure only shows up against a slow upstream.
func TestInferenceCallsDoNotUseRestClient(t *testing.T) {
	// Functions that wait on a model. Anything else (model lists, quota,
	// billing, token refresh) is metadata and belongs on the REST client.
	inference := regexp.MustCompile(`^func (Call|do[A-Za-z]*Request|doRequest|callResponses)`)

	root := ".."
	var offenders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		if !strings.Contains(path, string(filepath.Separator)+"providers"+string(filepath.Separator)) {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var current string
		for i, line := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(line, "func ") {
				current = line
			}
			if strings.Contains(line, "GetRestClientForAccount") && inference.MatchString(current) {
				offenders = append(offenders, path+":"+itoa(i+1)+" in "+firstWords(current))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("inference on the REST client:\n  %s", strings.Join(offenders, "\n  "))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func firstWords(s string) string {
	if i := strings.Index(s, "("); i > 0 {
		return s[:i]
	}
	return s
}

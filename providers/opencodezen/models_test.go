package opencodezen

import "testing"

func TestIsFreeModel(t *testing.T) {
	for _, id := range []string{"jev-1.13-free", "mimo-v2.5-free", zenStaticModels[0].ModelId} {
		if !isFreeModel(id) {
			t.Errorf("%s should be free", id)
		}
	}
	for _, id := range []string{"gpt-5.6-sol", "glm-5.3-flash", "muse-spark-1.3", "claude-opus-5"} {
		if isFreeModel(id) {
			t.Errorf("%s should be paid", id)
		}
	}
}

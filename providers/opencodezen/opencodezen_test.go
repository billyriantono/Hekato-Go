package opencodezen

import (
	"encoding/json"
	"strings"
	"testing"

	"hekato-go/providers"
)

func TestMarshalChatRequestMatchesOpenCodeCLIFrame(t *testing.T) {
	req := &providers.OpenAIRequest{Model: "jev-1.13-free"}
	req.Messages = []providers.OpenAIMessage{{Role: "user", Content: "OK"}}
	req.Tools, _ = InjectFingerprintTools(req.Tools)

	body, err := marshalChatRequest(req, true)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Stream        bool   `json:"stream"`
		ToolChoice    string `json:"tool_choice"`
		StreamOptions struct {
			IncludeUsage bool `json:"include_usage"`
		} `json:"stream_options"`
		Tools []struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Stream || !got.StreamOptions.IncludeUsage || got.ToolChoice != "none" {
		t.Fatalf("unexpected OpenCode frame: %s", body)
	}
	if len(got.Tools) != 2 || got.Tools[0].Function.Name != "bash" || got.Tools[1].Function.Name != "read" {
		t.Fatalf("unexpected required tools: %s", body)
	}
}

func TestInjectFingerprintToolsUsesCLIRequiredPair(t *testing.T) {
	tools, _ := InjectFingerprintTools(nil)
	if len(tools) != 2 || tools[0].Function.Name != "bash" || tools[1].Function.Name != "read" {
		t.Fatalf("required tools = %#v, want bash and read", tools)
	}
}

func TestStaticModelsExcludeSystemOneJev(t *testing.T) {
	for _, model := range zenStaticModels {
		if model.ModelId == "jev-1.13-free" || model.ModelId == "jev-1.13" {
			t.Fatalf("SystemOne model %q must not be exposed as a chat model", model.ModelId)
		}
	}
}

func TestMuseSparkUsesResponsesTransport(t *testing.T) {
	if !usesResponsesTransport("muse-spark-1.3-contributor-free") || !usesResponsesTransport(" Muse-Spark-1.2 ") {
		t.Fatal("muse-spark must go over /responses")
	}
	if usesResponsesTransport("nemotron-3.5-lightning-free") {
		t.Fatal("chat models stay on /chat/completions")
	}
	body, err := marshalResponsesRequest(&providers.ResponsesRequest{Model: "muse-spark-1.3-contributor-free", Tools: InjectFingerprintResponsesTools(nil)}, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "tool_choice") || !strings.Contains(string(body), `"name":"bash"`) || !strings.Contains(string(body), `"stream":true`) {
		t.Fatalf("zen responses frame wrong: %s", body)
	}
	var seen []string
	cb := dropFingerprintCalls(&providers.StreamCallback{OnToolUse: func(tu providers.ToolUse) { seen = append(seen, tu.Name) }})
	cb.OnToolUse(providers.ToolUse{Name: "bash"})
	cb.OnToolUse(providers.ToolUse{Name: "Read"})
	cb.OnToolUse(providers.ToolUse{Name: "get_weather"})
	if len(seen) != 1 || seen[0] != "get_weather" {
		t.Fatalf("fingerprint calls must be dropped, got %v", seen)
	}
}

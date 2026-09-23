package opencodezen

import (
	"encoding/json"
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

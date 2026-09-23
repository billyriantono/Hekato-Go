package proxy

import (
	"encoding/json"
	"hekato-go/providers/kiro"
	"testing"
)

// TestOpenAIToolAcceptsResponsesFlatFormat verifies that the Responses API tool
// shape (name/description/parameters at the top level) is parsed correctly, not
// just the Chat Completions nested {"function":{...}} shape. Previously the flat
// form produced an empty Function.Name, which Kiro rejected with HTTP 400
// "Improperly formed request".
func TestOpenAIToolAcceptsResponsesFlatFormat(t *testing.T) {
	flat := `{"type":"function","name":"exec_command","description":"Run a shell command","parameters":{"type":"object","properties":{"cmd":{"type":"string"}}}}`
	var tool OpenAITool
	if err := json.Unmarshal([]byte(flat), &tool); err != nil {
		t.Fatalf("unmarshal flat tool: %v", err)
	}
	if tool.Function.Name != "exec_command" {
		t.Fatalf("expected name exec_command, got %q", tool.Function.Name)
	}
	if tool.Function.Description != "Run a shell command" {
		t.Fatalf("expected description preserved, got %q", tool.Function.Description)
	}
	if tool.Function.Parameters == nil {
		t.Fatalf("expected parameters preserved")
	}
}

// TestOpenAIToolAcceptsNestedFormat verifies the Chat Completions nested shape
// still works after adding flat-format support.
func TestOpenAIToolAcceptsNestedFormat(t *testing.T) {
	nested := `{"type":"function","function":{"name":"get_weather","description":"Get weather","parameters":{"type":"object"}}}`
	var tool OpenAITool
	if err := json.Unmarshal([]byte(nested), &tool); err != nil {
		t.Fatalf("unmarshal nested tool: %v", err)
	}
	if tool.Function.Name != "get_weather" {
		t.Fatalf("expected name get_weather, got %q", tool.Function.Name)
	}
}

// TestConvertOpenAIToolsEmitsNonEmptyNames ensures the converter never emits a
// tool spec with an empty name (Kiro rejects those) and preserves valid names.
func TestConvertOpenAIToolsEmitsNonEmptyNames(t *testing.T) {
	tools := []OpenAITool{
		mustTool(t, `{"type":"function","name":"exec_command","parameters":{"type":"object"}}`),
		mustTool(t, `{"type":"function","function":{"name":"update_plan","parameters":{"type":"object"}}}`),
	}
	wrappers, nameMap := kiro.ConvertTools(openAIToNeutral(&OpenAIRequest{Tools: tools}).Tools)
	if len(wrappers) != 2 {
		t.Fatalf("expected 2 tool wrappers, got %d", len(wrappers))
	}
	for i, w := range wrappers {
		if w.ToolSpecification.Name == "" {
			t.Fatalf("tool %d has empty name", i)
		}
	}
	// Names are sanitized to camelCase for Kiro and mapped back for the client.
	if wrappers[0].ToolSpecification.Name != "execCommand" || nameMap["execCommand"] != "exec_command" {
		t.Fatalf("expected execCommand mapped to exec_command, got %q / %v", wrappers[0].ToolSpecification.Name, nameMap)
	}
	if wrappers[1].ToolSpecification.Name != "updatePlan" || nameMap["updatePlan"] != "update_plan" {
		t.Fatalf("expected updatePlan mapped to update_plan, got %q / %v", wrappers[1].ToolSpecification.Name, nameMap)
	}
}

func mustTool(t *testing.T, s string) OpenAITool {
	t.Helper()
	var tool OpenAITool
	if err := json.Unmarshal([]byte(s), &tool); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return tool
}

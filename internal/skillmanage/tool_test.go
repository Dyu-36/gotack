package skillmanage

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/mcp"
)

func TestToolsExposeMutationAndSafetySurface(t *testing.T) {
	tools := Tools(newTestManager(t))
	if len(tools) != 2 {
		t.Fatalf("tool count = %d, want 2", len(tools))
	}
	byName := make(map[string]mcp.Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = tool
		if strings.Contains(string(tool.Schema), "_session_id") || strings.Contains(string(tool.Schema), "_background_review") {
			t.Fatalf("hidden host metadata leaked in %s schema: %s", tool.Name, tool.Schema)
		}
	}
	if _, ok := byName[ManageToolName]; !ok {
		t.Fatalf("missing tool %q", ManageToolName)
	}
	if _, ok := byName[ViewToolName]; !ok {
		t.Fatalf("missing tool %q", ViewToolName)
	}

	var schema struct {
		Properties map[string]struct {
			Items struct {
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"items"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(byName[ManageToolName].Schema, &schema); err != nil {
		t.Fatal(err)
	}
	got := schema.Properties["operations"].Items.Properties["action"].Enum
	want := []string{actionCreate, actionPatch, actionDelete, actionWriteFile, actionRemoveFile}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("manage actions = %v, want %v", got, want)
	}
	if strings.Contains(string(byName[ManageToolName].Schema), `"edit"`) {
		t.Fatalf("obsolete edit action leaked: %s", byName[ManageToolName].Schema)
	}
}

func TestHiddenOriginFieldsDriveOwnershipAndReadMarks(t *testing.T) {
	manager := newTestManager(t)
	tools := toolMap(Tools(manager))
	content := skillText("reviewed", "Use when testing hidden context.", "Original.")
	raw, err := tools[ManageToolName].Handler(context.Background(), json.RawMessage(fmtJSON(t, map[string]any{
		"operations":  []map[string]any{{"action": actionCreate, "name": "reviewed", "content": content}},
		"_session_id": "review-session", "_background_review": true,
	})))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONSuccess(t, raw)

	raw, err = tools[ViewToolName].Handler(context.Background(), json.RawMessage(fmtJSON(t, map[string]any{
		"name": "reviewed", "_session_id": "review-session", "_background_review": true,
	})))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONSuccess(t, raw)

	replacement := "Updated."
	raw, err = tools[ManageToolName].Handler(context.Background(), json.RawMessage(fmtJSON(t, map[string]any{
		"operations":  []map[string]any{{"action": actionPatch, "name": "reviewed", "old_string": "Original.", "new_string": replacement}},
		"_session_id": "review-session", "_background_review": true,
	})))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONSuccess(t, raw)
}

func TestToolRejectsUnknownArgumentsAsStructuredFailure(t *testing.T) {
	tool := toolMap(Tools(newTestManager(t)))[ManageToolName]
	raw, err := tool.Handler(context.Background(), json.RawMessage(`{"operations":[],"invented":true}`))
	if err != nil {
		t.Fatalf("failure escaped as MCP error: %v", err)
	}
	var result Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	if result.Success || !strings.Contains(result.Error, "unknown field") {
		t.Fatalf("result = %s", raw)
	}
}

func toolMap(tools []mcp.Tool) map[string]mcp.Tool {
	result := make(map[string]mcp.Tool, len(tools))
	for _, tool := range tools {
		result[tool.Name] = tool
	}
	return result
}

func fmtJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertJSONSuccess(t *testing.T, raw string) {
	t.Helper()
	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("tool failed: %s", result.Error)
	}
}

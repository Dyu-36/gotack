package skillmanage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Dyu-36/gotack/internal/mcp"
)

const ManageToolName = "skill_manage"

type manageRequest struct {
	Operations       []Operation `json:"operations"`
	SessionID        string      `json:"_session_id,omitempty"`
	BackgroundReview bool        `json:"_background_review,omitempty"`
}

func Tools(manager *Manager) []mcp.Tool {
	return []mcp.Tool{viewTool(manager), manageTool(manager)}
}

func manageTool(manager *Manager) mcp.Tool {
	return mcp.Tool{
		Name: ManageToolName,
		Description: "Create or update procedural skills atomically. A single mutation is an operations array of one; " +
			"for an existing target, understand it with Crush view and call skill_view immediately before changing it.",
		Schema: json.RawMessage(`{
			"type":"object",
			"additionalProperties":false,
			"properties":{
				"operations":{
					"type":"array",
					"maxItems":20,
					"items":{
						"type":"object",
						"additionalProperties":false,
						"properties":{
							"action":{"type":"string","enum":["create","patch","delete","write_file","remove_file"]},
							"name":{"type":"string","description":"Lowercase letters, numbers, and hyphens; max 64 characters."},
							"content":{"type":"string","description":"Complete SKILL.md for create."},
							"category":{"type":"string","description":"Optional single category directory for create."},
							"old_string":{"type":"string","description":"Exact unique text to replace for patch."},
							"new_string":{"type":"string","description":"Replacement for patch; empty deletes the match."},
							"file_path":{"type":"string","description":"Optional patch target, or required support-file target, below references/, templates/, scripts/, or assets/."},
							"file_content":{"type":"string","description":"Complete support-file content for write_file."},
							"absorbed_into":{"type":"string","description":"Required only for background-review delete: existing agent-owned umbrella that received this skill's content."}
						},
						"required":["action","name"]
					}
				}
			},
			"required":["operations"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var request manageRequest
			if err := decodeStrict(args, &request); err != nil {
				return encodeResult(failed(fmt.Errorf("decode arguments: %w", err)))
			}
			return encodeResult(manager.ApplyWithMeta(ctx, request.Operations, RequestMeta{
				SessionID:        request.SessionID,
				BackgroundReview: request.BackgroundReview,
			}))
		},
	}
}

func decodeStrict(args json.RawMessage, target any) error {
	if len(bytes.TrimSpace(args)) == 0 {
		args = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func encodeResult(result any) (string, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode skill tool result: %w", err)
	}
	return string(data), nil
}

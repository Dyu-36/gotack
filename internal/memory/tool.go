package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Dyu-36/gotack/internal/mcp"
)

const ToolName = "memory"

var toolDescription = fmt.Sprintf(
	"Curate bounded persistent memory already visible in the system prompt. Prefer one atomic operations batch for multiple changes or consolidation. Each item uses add, whole-entry replace, or remove; replace/remove identify one entry with a unique old_text substring. Targets: memory (stable environment facts, %d chars) and user (identity/preferences, %d chars). Successful writes are terminal; do not repeat them. Skip task progress, raw dumps, transient facts, and reusable procedures (those belong in a skill).",
	MemoryCap, UserCap,
)

var toolSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"action": {"type": "string", "enum": ["add", "replace", "remove"], "description": "Single-operation shape; omit when using operations."},
		"target": {"type": "string", "enum": ["memory", "user"], "description": "memory for durable notes; user for user profile."},
		"content": {"type": "string", "description": "Entry for add/replace."},
		"new_text": {"type": "string", "description": "Alias for content."},
		"old_text": {"type": "string", "description": "Unique substring required for replace/remove."},
		"operations": {
			"type": "array",
			"description": "Preferred atomic all-or-nothing batch; final state alone is checked against the character cap.",
			"items": {
				"type": "object",
				"properties": {
					"action": {"type": "string", "enum": ["add", "replace", "remove"]},
					"content": {"type": "string"},
					"new_text": {"type": "string"},
					"old_text": {"type": "string"}
				},
				"required": ["action"]
			}
		}
	},
	"required": ["target"]
}`)

func Tool(store *Store) mcp.Tool {
	return mcp.Tool{
		Name:        ToolName,
		Description: toolDescription,
		Schema:      toolSchema,
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			req, err := decodeArgs(args)
			if err != nil {
				return encodeFailure(err), nil
			}
			result, err := dispatch(ctx, store, req)
			if err != nil {
				return encodeFailure(err), nil
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				return "", fmt.Errorf("memory: encode result: %w", err)
			}
			return string(encoded), nil
		},
	}
}

type request struct {
	Action     string      `json:"action"`
	Target     string      `json:"target"`
	Content    string      `json:"content"`
	NewText    string      `json:"new_text"`
	OldText    string      `json:"old_text"`
	Operations []Operation `json:"operations"`
}

func decodeArgs(args json.RawMessage) (request, error) {
	if len(args) == 0 {
		return request{}, fmt.Errorf("arguments are required: %w", ErrArguments)
	}
	var req request
	if err := json.Unmarshal(args, &req); err != nil {
		return request{}, fmt.Errorf("decode arguments: %v: %w", err, ErrArguments)
	}
	return req, nil
}

func dispatch(ctx context.Context, store *Store, req request) (Result, error) {
	target := Target(strings.TrimSpace(req.Target))
	if target == "" {
		target = TargetMemory
	}
	if target != TargetMemory && target != TargetUser {
		return Result{}, fmt.Errorf("target %q: %w", req.Target, ErrUnknownTarget)
	}
	if req.Operations != nil {
		return store.Apply(ctx, target, req.Operations)
	}

	content := req.Content
	if content == "" {
		content = req.NewText
	}
	operation := Operation{
		Action:  strings.TrimSpace(req.Action),
		Content: content,
		OldText: req.OldText,
	}
	return store.Apply(ctx, target, []Operation{operation})
}

type failureResult struct {
	Success        bool     `json:"success"`
	Error          string   `json:"error"`
	CurrentEntries []string `json:"current_entries,omitempty"`
	Usage          string   `json:"usage,omitempty"`
}

func encodeFailure(err error) string {
	result := failureResult{Success: false, Error: err.Error()}

	var overCap *OverCapError
	if errors.As(err, &overCap) {
		result.CurrentEntries = overCap.Entries
		result.Usage = fmt.Sprintf("%s/%s", group(overCap.Used), group(overCap.Cap))
	} else {
		var operation *operationError
		if errors.As(err, &operation) && (errors.Is(err, ErrTextNotFound) || errors.Is(err, ErrTextNotUnique) || errors.Is(err, ErrMissingOldText)) {
			result.CurrentEntries = operation.entries
			result.Usage = fmt.Sprintf("%s/%s", group(operation.used), group(operation.cap))
		}
	}

	encoded, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return `{"success":false,"error":"memory: could not encode error"}`
	}
	return string(encoded)
}

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

var toolDescription = fmt.Sprintf("Curate small persistent personal context. Targets: profile (identity/preferences, %d characters) and memory (key durable facts, %d characters). Legacy target user means profile. Use one atomic operations batch to consolidate whole entries. Do not increase limits or save transcripts, task progress, temporary errors, secrets, or procedures. Retrieve older details with session_search. A successful write is complete; do not repeat it.", ProfileCap, MemoryCap)

var toolSchema = json.RawMessage(`{
 "type":"object","properties":{
 "action":{"type":"string","enum":["add","replace","remove"]},
 "target":{"type":"string","enum":["profile","memory","user"]},
 "content":{"type":"string","description":"Concise whole entry for add/replace."},
 "new_text":{"type":"string","description":"Alias for content."},
 "old_text":{"type":"string","description":"Unique substring for replace/remove."},
 "operations":{"type":"array","maxItems":32,"description":"Atomic batch; consolidate instead of growing the limit.","items":{"type":"object","properties":{
 "action":{"type":"string","enum":["add","replace","remove"]},"content":{"type":"string"},"new_text":{"type":"string"},"old_text":{"type":"string"}},"required":["action"]}}
 },"required":["target"]
}`)

func Tool(store *Store) mcp.Tool {
	return mcp.Tool{
		Name: ToolName, Description: toolDescription, Schema: toolSchema,
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
	if len(args) > MaxPromptFileBytes {
		return request{}, ErrPromptFileTooLarge
	}
	var req request
	if err := json.Unmarshal(args, &req); err != nil {
		return request{}, fmt.Errorf("decode arguments: %v: %w", err, ErrArguments)
	}
	return req, nil
}

func dispatch(ctx context.Context, store *Store, req request) (Result, error) {
	target := canonicalTarget(Target(strings.TrimSpace(req.Target)))
	if target == "" {
		target = TargetMemory
	}
	if target != TargetMemory && target != TargetProfile {
		return Result{}, fmt.Errorf("target %q: %w", req.Target, ErrUnknownTarget)
	}
	if req.Operations != nil {
		return store.Apply(ctx, target, req.Operations)
	}
	content := req.Content
	if content == "" {
		content = req.NewText
	}
	return store.Apply(ctx, target, []Operation{{
		Action: strings.TrimSpace(req.Action), Content: content, OldText: req.OldText,
	}})
}

type failureResult struct {
	Success        bool     `json:"success"`
	Error          string   `json:"error"`
	CurrentEntries []string `json:"current_entries,omitempty"`
	Usage          string   `json:"usage,omitempty"`
}

func encodeFailure(err error) string {
	result := failureResult{Success: false, Error: truncateRunes(err.Error(), 512)}
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
	// Do not echo an externally enlarged file wholesale into the conversation.
	remaining := 4 * (MemoryCap + ProfileCap)
	bounded := make([]string, 0, len(result.CurrentEntries))
	for _, entry := range result.CurrentEntries {
		if len(entry) > remaining {
			continue
		}
		bounded = append(bounded, entry)
		remaining -= len(entry)
	}
	result.CurrentEntries = bounded
	encoded, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return `{"success":false,"error":"memory: could not encode error"}`
	}
	return string(encoded)
}

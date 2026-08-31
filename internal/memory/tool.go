package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Dyu-36/gotack/internal/mcp"
)

// ToolName is the single MCP tool exposed by the memory server, named
// after the Phase 2 plan.
const ToolName = "memory"

// Action and target vocabulary. Kept as plain constants so the validation
// matrix and the schema stay visibly in one place.
const (
	actionView    = "view"
	actionAdd     = "add"
	actionReplace = "replace"
	actionRemove  = "remove"

	toolDescription = "View and curate the assistant's persistent memory. Two files: " +
		"\"memory\" (MEMORY.md, durable facts) and \"user\" (USER.md, user preferences), " +
		"organised in §-delimited sections. Actions: view (read one file), add (append " +
		"content to a section, creating it if absent), replace (swap one existing " +
		"section), remove (drop one existing section). Writes are size-capped " +
		"(MEMORY.md 2200 bytes, USER.md 1375 bytes); when a write overflows the cap " +
		"the oldest entries are evicted, and an entry that alone exceeds the cap is " +
		"rejected so you must consolidate. Every result reports size, cap and " +
		"remaining budget. Content is re-injected into the system prompt on the next turn."
)

// Tool builds the memory MCP tool over store.
func Tool(store *Store) mcp.Tool {
	return mcp.Tool{
		Name:        ToolName,
		Description: toolDescription,
		Schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"action": {"type": "string", "enum": ["view", "add", "replace", "remove"], "description": "view: read one file; add: append to a section (creates it); replace: swap one existing section; remove: drop one existing section"},
				"target": {"type": "string", "enum": ["memory", "user"], "description": "Which file: \"memory\" (durable facts, default) or \"user\" (user preferences)"},
				"section": {"type": "string", "description": "Section heading (§-delimited); required for add, replace and remove"},
				"content": {"type": "string", "description": "Text to store; required for add and replace"}
			},
			"required": ["action"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			req, err := decodeArgs(args)
			if err != nil {
				return "", err
			}
			result, err := dispatch(ctx, store, req)
			if err != nil {
				return "", err
			}
			out, err := json.Marshal(result)
			if err != nil {
				return "", fmt.Errorf("memory: encode result: %w", err)
			}
			return string(out), nil
		},
	}
}

// request is the decoded tool call.
type request struct {
	Action  string `json:"action"`
	Target  string `json:"target"`
	Section string `json:"section"`
	Content string `json:"content"`
}

// decodeArgs validates the raw arguments shape before any dispatch.
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

// dispatch validates the vocabulary and required fields, then runs the
// operation. Unknown actions and targets are typed errors, and replace or
// remove against a missing section fail loudly — constrained by
// construction per decision 0003.
func dispatch(ctx context.Context, store *Store, req request) (Result, error) {
	action := strings.TrimSpace(req.Action)
	switch action {
	case actionView, actionAdd, actionReplace, actionRemove:
	default:
		return Result{}, fmt.Errorf("action %q: %w", req.Action, ErrUnknownAction)
	}

	target := Target(strings.TrimSpace(req.Target))
	if target == "" {
		target = TargetMemory
	}
	switch target {
	case TargetMemory, TargetUser:
	default:
		return Result{}, fmt.Errorf("target %q: %w", req.Target, ErrUnknownTarget)
	}

	section := strings.TrimSpace(req.Section)
	content := strings.TrimRight(req.Content, "\n")

	switch action {
	case actionView:
		return store.View(ctx, target)
	case actionAdd:
		if section == "" {
			return Result{}, fmt.Errorf("action %q: %w", action, ErrMissingSection)
		}
		if strings.TrimSpace(content) == "" {
			return Result{}, fmt.Errorf("action %q: %w", action, ErrEmptyContent)
		}
		return store.Add(ctx, target, section, content)
	case actionReplace:
		if section == "" {
			return Result{}, fmt.Errorf("action %q: %w", action, ErrMissingSection)
		}
		if strings.TrimSpace(content) == "" {
			return Result{}, fmt.Errorf("action %q: %w", action, ErrEmptyContent)
		}
		return store.Replace(ctx, target, section, content)
	default: // actionRemove
		if section == "" {
			return Result{}, fmt.Errorf("action %q: %w", action, ErrMissingSection)
		}
		return store.Remove(ctx, target, section)
	}
}

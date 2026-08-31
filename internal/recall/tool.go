package recall

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Dyu-36/gotack/internal/mcp"
)

// ToolName is the single MCP tool exposed by the recall server, named after
// the Phase 3 plan.
const ToolName = "session_search"

// Tool builds the session_search MCP tool over store. The handler syncs the
// index incrementally before each search, so a running engine's newest
// messages become recallable without any push wiring.
func Tool(store *Store) mcp.Tool {
	return mcp.Tool{
		Name:        ToolName,
		Description: "Search this assistant's past Crush conversations across all sessions. Use it to recall what was discussed or decided before. Every hit cites its source: session id, session title, role and timestamp.",
		Schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {"type": "string", "description": "Words or phrases to search for in past conversations"},
				"limit": {"type": "integer", "description": "Maximum number of results (default 10, max 50)"}
			},
			"required": ["query"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var req struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if len(args) == 0 {
				return "", fmt.Errorf("recall: arguments are required")
			}
			if err := json.Unmarshal(args, &req); err != nil {
				return "", fmt.Errorf("recall: decode arguments: %w", err)
			}
			results, err := store.Search(ctx, req.Query, req.Limit)
			if err != nil {
				return "", err
			}
			payload := struct {
				Count   int      `json:"count"`
				Results []Result `json:"results"`
			}{Count: len(results), Results: results}
			if payload.Results == nil {
				payload.Results = []Result{}
			}
			out, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				return "", fmt.Errorf("recall: encode results: %w", err)
			}
			return string(out), nil
		},
	}
}

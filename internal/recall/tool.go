package recall

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/mcp"
)

const ToolName = "session_search"

const toolDescription = "Find relevant past conversations in the local database, without an LLM. Query returns brief snippets and message IDs by default. Use session_id plus around_message_id for a bounded window, or session_id alone for a bounded session read. No arguments browses recent sessions. Results are size-limited; follow IDs instead of requesting the whole history."

const toolSchema = `{"type":"object","properties":{
 "query":{"type":"string","description":"Names, keywords, or an FTS5 expression. Omit to browse."},
 "limit":{"type":"integer","description":"Maximum sessions (default 3, max 10).","default":3},
 "sort":{"type":"string","enum":["newest","oldest"]},
 "detail":{"type":"string","enum":["brief","adaptive","full"],"description":"Brief returns snippets only. Adaptive/full expand windows subject to the same output cap.","default":"brief"},
 "session_id":{"type":"string"},
 "around_message_id":{"type":"string","description":"Use match_message_id from discovery to read around a match."},
 "window":{"type":"integer","description":"Messages each side (default 5, max 20).","default":5},
 "role_filter":{"type":"string","description":"Comma-separated roles; defaults to user,assistant."}
},"required":[]}`

type request struct {
	Query            string  `json:"query"`
	RoleFilter       string  `json:"role_filter"`
	Limit            *int    `json:"limit"`
	SessionID        string  `json:"session_id"`
	AroundMessageID  *string `json:"around_message_id"`
	Window           *int    `json:"window"`
	Sort             string  `json:"sort"`
	Detail           string  `json:"detail"`
	CurrentSessionID string  `json:"current_session_id"`
}

func Tool(store *Store) mcp.Tool {
	return mcp.Tool{
		Name: ToolName, Description: toolDescription, Schema: json.RawMessage(toolSchema),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("recall: send {} to browse sessions")
			}
			if len(args) > 16*1024 {
				return "", fmt.Errorf("recall: arguments exceed the size limit")
			}
			var req request
			if err := json.Unmarshal(args, &req); err != nil {
				return "", fmt.Errorf("recall: decode arguments: %w", err)
			}
			started := time.Now()
			result, err := dispatch(ctx, store, req)
			// Counts and timings only; no user query, snippets, or session IDs.
			store.log.Info("recall output budget", "response_bytes", len(result),
				"limit_bytes", maxToolResponseBytes, "elapsed_us", time.Since(started).Microseconds(), "failed", err != nil)
			return result, err
		},
	}
}

func dispatch(ctx context.Context, store *Store, req request) (string, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	if req.AroundMessageID != nil {
		anchorID := strings.TrimSpace(*req.AroundMessageID)
		if sessionID == "" || anchorID == "" {
			return "", fmt.Errorf("recall: around_message_id requires a non-empty session_id and message id")
		}
		result, err := store.Around(ctx, sessionID, anchorID, requestWindow(req.Window))
		if err != nil {
			return "", err
		}
		return encode(struct {
			Success bool   `json:"success"`
			Mode    string `json:"mode"`
			AroundResult
		}{true, "scroll", result})
	}
	if sessionID != "" {
		result, err := store.ReadSession(ctx, sessionID)
		if err != nil {
			return "", err
		}
		return encode(struct {
			Success bool   `json:"success"`
			Mode    string `json:"mode"`
			ReadResult
		}{true, "read", result})
	}
	query := strings.TrimSpace(req.Query)
	if query != "" {
		detail := parseDetail(req.Detail)
		results, err := store.SearchWithOptions(ctx, SearchOptions{
			Query: query, Roles: splitRoles(req.RoleFilter), Limit: requestLimit(req.Limit),
			Sort: parseSortOrder(req.Sort), Detail: detail, ExcludeSessionID: strings.TrimSpace(req.CurrentSessionID),
		})
		if err != nil {
			return "", err
		}
		return encode(struct {
			Success          bool              `json:"success"`
			Mode             string            `json:"mode"`
			Query            string            `json:"query"`
			Detail           Detail            `json:"detail"`
			Results          []DiscoveryResult `json:"results"`
			Count            int               `json:"count"`
			SessionsSearched int               `json:"sessions_searched"`
		}{true, "discover", query, detail, results, len(results), len(results)})
	}
	results, err := store.BrowseWithOptions(ctx, requestLimit(req.Limit), strings.TrimSpace(req.CurrentSessionID))
	if err != nil {
		return "", err
	}
	return encode(struct {
		Success bool             `json:"success"`
		Mode    string           `json:"mode"`
		Results []SessionSummary `json:"results"`
		Count   int              `json:"count"`
	}{true, "browse", results, len(results)})
}

func splitRoles(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func requestLimit(value *int) int {
	if value == nil {
		return defaultResultLimit
	}
	if *value < 1 {
		return 1
	}
	if *value > maxResultLimit {
		return maxResultLimit
	}
	return *value
}

func requestWindow(value *int) int {
	if value == nil {
		return defaultAroundWindow
	}
	if *value < 1 {
		return 1
	}
	if *value > maxAroundWindow {
		return maxAroundWindow
	}
	return *value
}

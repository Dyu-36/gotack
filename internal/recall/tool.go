package recall

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Dyu-36/gotack/internal/mcp"
)

const ToolName = "session_search"

const toolDescription = "Search past Crush sessions or read inside one. Four shapes are selected by arguments: " +
	"query discovers matching sessions (the top result is fully hydrated by default); " +
	"session_id plus around_message_id returns a bounded window around that message; " +
	"session_id alone reads the session (large sessions return the first 20 and last 10 messages); " +
	"no arguments browses recent sessions. Results are actual local database messages; no LLM is used."

const toolSchema = `{"type":"object","properties":{
	"query":{"type":"string","description":"FTS5 discovery query. Omit to browse recent sessions."},
	"limit":{"type":"integer","description":"Maximum sessions for discovery or browse (default 3, max 10).","default":3},
	"sort":{"type":"string","enum":["newest","oldest"],"description":"Optional temporal ordering for discovery; omit for relevance."},
	"detail":{"type":"string","enum":["adaptive","full"],"description":"Adaptive fully hydrates only the top discovery result; full hydrates all.","default":"adaptive"},
	"session_id":{"type":"string","description":"Session to read, or to scroll when paired with around_message_id."},
	"around_message_id":{"type":"string","description":"Crush message id to center the scroll window on."},
	"window":{"type":"integer","description":"Messages on each side of the anchor (default 5, clamped to 1-20).","default":5},
	"role_filter":{"type":"string","description":"Comma-separated discovery roles. Defaults to user,assistant; tool is also accepted."}
},"required":[]}`

type request struct {
	Query           string  `json:"query"`
	RoleFilter      string  `json:"role_filter"`
	Limit           *int    `json:"limit"`
	SessionID       string  `json:"session_id"`
	AroundMessageID *string `json:"around_message_id"`
	Window          *int    `json:"window"`
	Sort            string  `json:"sort"`
	Detail          string  `json:"detail"`
	// CurrentSessionID is an engine-injected value, not a model argument. The
	// Crush adapter overwrites it from the trusted session context so discovery
	// does not return the conversation already in the model's prompt.
	CurrentSessionID string `json:"current_session_id"`
}

func Tool(store *Store) mcp.Tool {
	return mcp.Tool{
		Name:        ToolName,
		Description: toolDescription,
		Schema:      json.RawMessage(toolSchema),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("recall: arguments are required; send {} to browse sessions")
			}
			var req request
			if err := json.Unmarshal(args, &req); err != nil {
				return "", fmt.Errorf("recall: decode arguments: %w", err)
			}
			return dispatch(ctx, store, req)
		},
	}
}

// dispatch follows Hermes' precedence: anchored scroll, session read, FTS
// discovery, then no-argument browse.
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
			Query:            query,
			Roles:            splitRoles(req.RoleFilter),
			Limit:            requestLimit(req.Limit),
			Sort:             parseSortOrder(req.Sort),
			Detail:           detail,
			ExcludeSessionID: strings.TrimSpace(req.CurrentSessionID),
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

func encode(payload any) (string, error) {
	out, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("recall: encode results: %w", err)
	}
	return string(out), nil
}

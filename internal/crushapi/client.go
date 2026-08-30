// client.go -- role: request and response calls against the Crush server.
//
// Sessions, messages, permissions and questions. One method per route and no
// business logic: callers in internal/ decide what to do with the result.
package crushapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// Route templates. The Crush server uses Go 1.22+ mux syntax ({id}, {sid});
// we substitute the values locally because the bridge speaks HTTP without
// a host.
const (
	healthPath          = "/v1/health"
	versionPath         = "/v1/version"
	workspacesPath      = "/v1/workspaces"
	permissionGrantPath = "/v1/workspaces/{id}/permissions/grant"
	permissionSkipPath  = "/v1/workspaces/{id}/permissions/skip"
	questionsAnswerPath = "/v1/workspaces/{id}/questions/answer"
	agentPath           = "/v1/workspaces/{id}/agent"
	agentInitPath       = "/v1/workspaces/{id}/agent/init"
	cancelPath          = "/v1/workspaces/{id}/agent/sessions/{sid}/cancel"
	currentSessionPath  = "/v1/workspaces/{id}/current-session"
	sessionsPath        = "/v1/workspaces/{id}/sessions"
	messagesPath        = "/v1/workspaces/{id}/sessions/{sid}/messages"
	historyPath         = "/v1/workspaces/{id}/sessions/{sid}/history"
	eventsPath          = "/v1/workspaces/{id}/events"
)

// Client is a thin HTTP wrapper around a Crush server reachable through a
// pre-dialed transport. The http.Client is shared across all calls; the
// clientID identifies this consumer in SSE subscription queries.
type Client struct {
	hc       *http.Client
	clientID string
}

// NewClient returns a Client whose clientID is a fresh UUID v4. The http
// transport must already be wired to the Crush pipe or socket by Dial.
func NewClient(hc *http.Client) *Client {
	return &Client{hc: hc, clientID: uuid.NewString()}
}

// ID returns the UUID identifying this Client in stream subscriptions.
func (c *Client) ID() string { return c.clientID }

// Ping issues GET /v1/health and returns nil on a 2xx response. Used by the
// engine supervisor to probe a running server before adopting it.
func Ping(hc *http.Client, ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL(healthPath), nil)
	if err != nil {
		return err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return decodeError(resp)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// Version calls GET /v1/version.
func (c *Client) Version(ctx context.Context) (VersionInfo, error) {
	var v VersionInfo
	if err := c.doJSON(ctx, http.MethodGet, versionPath, nil, &v); err != nil {
		return VersionInfo{}, err
	}
	return v, nil
}

// ListWorkspaces calls GET /v1/workspaces.
func (c *Client) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	var ws []Workspace
	if err := c.doJSON(ctx, http.MethodGet, workspacesPath, nil, &ws); err != nil {
		return nil, err
	}
	return ws, nil
}

// CreateWorkspace calls POST /v1/workspaces with the default Crush data
// directory derived from the workspace path.
func (c *Client) CreateWorkspace(ctx context.Context, path string, yolo bool) (Workspace, error) {
	return c.CreateWorkspaceWithDataDir(ctx, path, "", yolo)
}

// CreateWorkspaceWithDataDir creates a workspace while allowing Gotack to keep
// Crush's database/config state outside the selected working directory. This is
// especially important for the default C:\ workspace, where writing C:\.crush
// may require elevated Windows permissions.
func (c *Client) CreateWorkspaceWithDataDir(ctx context.Context, path, dataDir string, yolo bool) (Workspace, error) {
	body, _ := json.Marshal(struct {
		Path     string `json:"path"`
		DataDir  string `json:"data_dir,omitempty"`
		YOLO     bool   `json:"yolo"`
		ClientID string `json:"client_id"`
	}{Path: path, DataDir: dataDir, YOLO: yolo, ClientID: c.clientID})
	var ws Workspace
	if err := c.doJSON(ctx, http.MethodPost, workspacesPath+"?client_id="+url.QueryEscape(c.clientID), bytes.NewReader(body), &ws); err != nil {
		return Workspace{}, err
	}
	return ws, nil
}

// SetCurrentSession records sessionID as the active session of this
// Client's clientID for the given workspace. The body shape matches
// proto.CurrentSession.
func (c *Client) SetCurrentSession(ctx context.Context, wsID, sessionID string) error {
	body, _ := json.Marshal(struct {
		SessionID string `json:"session_id"`
	}{SessionID: sessionID})
	return c.doJSON(ctx, http.MethodPost, expandPath(currentSessionPath, "id", wsID)+"?client_id="+url.QueryEscape(c.clientID), bytes.NewReader(body), nil)
}

// ListSessions calls GET /v1/workspaces/{id}/sessions.
func (c *Client) ListSessions(ctx context.Context, wsID string) ([]Session, error) {
	var ss []Session
	if err := c.doJSON(ctx, http.MethodGet, expandPath(sessionsPath, "id", wsID), nil, &ss); err != nil {
		return nil, err
	}
	return ss, nil
}

// CreateSession calls POST /v1/workspaces/{id}/sessions with a {title} body.
func (c *Client) CreateSession(ctx context.Context, wsID, title string) (Session, error) {
	body, _ := json.Marshal(struct {
		Title string `json:"title"`
	}{Title: title})
	var s Session
	if err := c.doJSON(ctx, http.MethodPost, expandPath(sessionsPath, "id", wsID), bytes.NewReader(body), &s); err != nil {
		return Session{}, err
	}
	return s, nil
}

// Messages calls GET /v1/workspaces/{id}/sessions/{sid}/messages.
func (c *Client) Messages(ctx context.Context, wsID, sessionID string) ([]Message, error) {
	var ms []Message
	path := expandPath(messagesPath, "id", wsID, "sid", sessionID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &ms); err != nil {
		return nil, err
	}
	return ms, nil
}

// History calls GET /v1/workspaces/{id}/sessions/{sid}/history.
func (c *Client) History(ctx context.Context, wsID, sessionID string) ([]File, error) {
	var fs []File
	path := expandPath(historyPath, "id", wsID, "sid", sessionID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &fs); err != nil {
		return nil, err
	}
	return fs, nil
}

// SendPrompt posts an AgentMessage to /v1/workspaces/{id}/agent. The Crush
// server replies 202 once the prompt is queued; runID is echoed back in the
// RunComplete event so callers can correlate. An empty runID omits the
// field, which the server treats as "best effort" matching.
func (c *Client) SendPrompt(ctx context.Context, wsID, sessionID, text, runID string) error {
	body, _ := json.Marshal(struct {
		SessionID string `json:"session_id"`
		RunID     string `json:"run_id,omitempty"`
		Prompt    string `json:"prompt"`
	}{SessionID: sessionID, RunID: runID, Prompt: text})
	resp, err := c.do(ctx, http.MethodPost, expandPath(agentPath, "id", wsID), bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode/100 != 2 {
		return decodeError(resp)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// InitAgent initializes the workspace agent after model/provider configuration
// has been applied. Gotack uses the interactive agent because the UI handles
// permission and question requests emitted by Crush.
func (c *Client) InitAgent(ctx context.Context, wsID string, interactive bool) error {
	body, _ := json.Marshal(struct {
		Interactive bool `json:"interactive"`
	}{Interactive: interactive})
	path := expandPath(agentInitPath, "id", wsID)
	return c.doJSON(ctx, http.MethodPost, path, bytes.NewReader(body), nil)
}

// EnsureAgent initializes the workspace agent only when Crush reports it is
// not ready. Coordinators refresh model configuration before each run, so an
// already-ready agent must not be rebuilt merely because settings changed.
func (c *Client) EnsureAgent(ctx context.Context, wsID string, interactive bool) error {
	var info struct {
		IsReady bool `json:"is_ready"`
	}
	path := expandPath(agentPath, "id", wsID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &info); err != nil {
		return err
	}
	if info.IsReady {
		return nil
	}
	return c.InitAgent(ctx, wsID, interactive)
}

// CancelPrompt calls POST /v1/workspaces/{id}/agent/sessions/{sid}/cancel.
func (c *Client) CancelPrompt(ctx context.Context, wsID, sessionID string) error {
	path := expandPath(cancelPath, "id", wsID, "sid", sessionID)
	return c.doJSON(ctx, http.MethodPost, path, nil, nil)
}

// GrantPermission posts a PermissionGrant to /permissions/grant and
// returns whether the server reports the request as resolved by this call.
// A false Resolved means another caller already answered (not an error).
func (c *Client) GrantPermission(ctx context.Context, wsID string, req PermissionRequest, action PermissionAction) (bool, error) {
	body, _ := json.Marshal(PermissionGrant{Permission: req, Action: action})
	var resp struct {
		Resolved bool `json:"resolved"`
	}
	path := expandPath(permissionGrantPath, "id", wsID)
	if err := c.doJSON(ctx, http.MethodPost, path, bytes.NewReader(body), &resp); err != nil {
		return false, err
	}
	return resp.Resolved, nil
}

// SetPermissionsSkip enables or disables permission prompts for a workspace.
// Gotack enables this for every attached workspace so local file and tool
// access never blocks on an approval dialog.
func (c *Client) SetPermissionsSkip(ctx context.Context, wsID string, skip bool) error {
	body, _ := json.Marshal(struct {
		Skip bool `json:"skip"`
	}{Skip: skip})
	path := expandPath(permissionSkipPath, "id", wsID)
	return c.doJSON(ctx, http.MethodPost, path, bytes.NewReader(body), nil)
}

// AnswerQuestion posts a QuestionAnswer to /questions/answer and returns
// whether the server reports the batch as resolved by this call.
func (c *Client) AnswerQuestion(ctx context.Context, wsID string, ans QuestionAnswer) (bool, error) {
	body, _ := json.Marshal(ans)
	var resp struct {
		Resolved bool `json:"resolved"`
	}
	path := expandPath(questionsAnswerPath, "id", wsID)
	if err := c.doJSON(ctx, http.MethodPost, path, bytes.NewReader(body), &resp); err != nil {
		return false, err
	}
	return resp.Resolved, nil
}

// doJSON performs a request with the given body and decodes a JSON
// response into out. A nil body and nil out decode only the status code.
func (c *Client) doJSON(ctx context.Context, method, path string, body io.Reader, out any) error {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return decodeError(resp)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("crushapi: decode %s: %w", path, err)
	}
	return nil
}

// do sends a request and returns the response for the caller to consume.
// The transport dials the pipe/socket directly and ignores the host, but
// net/http requires an absolute URL, so requests target a fixed dummy base.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL(path), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.hc.Do(req)
}

// requestURL joins the dummy base with the API path (path may carry a query).
func requestURL(path string) string {
	return "http://crush" + path
}

// decodeError builds an error from a Crush error response. The server's
// body is {"message": "..."}; we surface status + message verbatim.
func decodeError(resp *http.Response) error {
	raw, _ := io.ReadAll(resp.Body)
	var e struct {
		Message string `json:"message"`
	}
	msg := ""
	if jerr := json.Unmarshal(raw, &e); jerr == nil {
		msg = e.Message
	}
	if msg == "" {
		msg = string(bytes.TrimSpace(raw))
	}
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("crushapi: %s %s: %d %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, msg)
}

// IsClientNotAttached reports Crush's presence error. The server requires a
// live workspace SSE subscription before current-session presence can be set.
func IsClientNotAttached(err error) bool {
	return err != nil && strings.Contains(err.Error(), "client not attached")
}

// expandPath substitutes Go-mux style placeholders ({name}) in template
// with the provided values. Values are url-path-escaped so client-supplied
// identifiers cannot break out of their segment.
func expandPath(template string, kv ...string) string {
	if len(kv) == 0 {
		return template
	}
	out := template
	for i := 0; i+1 < len(kv); i += 2 {
		out = strings.ReplaceAll(out, "{"+kv[i]+"}", url.PathEscape(kv[i+1]))
	}
	return out
}

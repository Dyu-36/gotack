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

const (
	healthPath          = "/v1/health"
	versionPath         = "/v1/version"
	workspacesPath      = "/v1/workspaces"
	permissionGrantPath = "/v1/workspaces/{id}/permissions/grant"
	permissionSkipPath  = "/v1/workspaces/{id}/permissions/skip"
	questionsAnswerPath = "/v1/workspaces/{id}/questions/answer"
	agentPath           = "/v1/workspaces/{id}/agent"
	agentInitPath       = "/v1/workspaces/{id}/agent/init"
	agentRefreshPath    = "/v1/workspaces/{id}/agent/refresh-prompt"
	cancelPath          = "/v1/workspaces/{id}/agent/sessions/{sid}/cancel"
	currentSessionPath  = "/v1/workspaces/{id}/current-session"
	sessionsPath        = "/v1/workspaces/{id}/sessions"
	messagesPath        = "/v1/workspaces/{id}/sessions/{sid}/messages"
	historyPath         = "/v1/workspaces/{id}/sessions/{sid}/history"
	eventsPath          = "/v1/workspaces/{id}/events"
)

type Client struct {
	hc       *http.Client
	clientID string
}

func NewClient(hc *http.Client) *Client {
	return &Client{hc: hc, clientID: uuid.NewString()}
}

func (c *Client) ID() string { return c.clientID }

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

func (c *Client) Version(ctx context.Context) (VersionInfo, error) {
	var v VersionInfo
	if err := c.doJSON(ctx, http.MethodGet, versionPath, nil, &v); err != nil {
		return VersionInfo{}, err
	}
	return v, nil
}

func (c *Client) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	var ws []Workspace
	if err := c.doJSON(ctx, http.MethodGet, workspacesPath, nil, &ws); err != nil {
		return nil, err
	}
	return ws, nil
}

func (c *Client) CreateWorkspace(ctx context.Context, path string, yolo bool) (Workspace, error) {
	return c.CreateWorkspaceWithDataDir(ctx, path, "", yolo)
}

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

func (c *Client) SetCurrentSession(ctx context.Context, wsID, sessionID string) error {
	body, _ := json.Marshal(struct {
		SessionID string `json:"session_id"`
	}{SessionID: sessionID})
	return c.doJSON(ctx, http.MethodPost, expandPath(currentSessionPath, "id", wsID)+"?client_id="+url.QueryEscape(c.clientID), bytes.NewReader(body), nil)
}

func (c *Client) ListSessions(ctx context.Context, wsID string) ([]Session, error) {
	var ss []Session
	if err := c.doJSON(ctx, http.MethodGet, expandPath(sessionsPath, "id", wsID), nil, &ss); err != nil {
		return nil, err
	}
	return ss, nil
}

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

func (c *Client) Messages(ctx context.Context, wsID, sessionID string) ([]Message, error) {
	var ms []Message
	path := expandPath(messagesPath, "id", wsID, "sid", sessionID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &ms); err != nil {
		return nil, err
	}
	return ms, nil
}

func (c *Client) History(ctx context.Context, wsID, sessionID string) ([]File, error) {
	var fs []File
	path := expandPath(historyPath, "id", wsID, "sid", sessionID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &fs); err != nil {
		return nil, err
	}
	return fs, nil
}

func (c *Client) SendPromptWithAttachments(ctx context.Context, wsID, sessionID, text, runID string, attachments []Attachment) error {
	return c.sendPromptWithAttachments(ctx, wsID, sessionID, text, runID, attachments, 0)
}

func (c *Client) SendPromptWithAttachmentsAndBudget(ctx context.Context, wsID, sessionID, text, runID string, attachments []Attachment, maxInputTokens int64) error {
	return c.sendPromptWithAttachments(ctx, wsID, sessionID, text, runID, attachments, maxInputTokens)
}

func (c *Client) sendPromptWithAttachments(ctx context.Context, wsID, sessionID, text, runID string, attachments []Attachment, maxInputTokens int64) error {
	body, _ := json.Marshal(struct {
		SessionID      string       `json:"session_id"`
		RunID          string       `json:"run_id,omitempty"`
		Prompt         string       `json:"prompt"`
		Attachments    []Attachment `json:"attachments,omitempty"`
		MaxInputTokens int64        `json:"max_input_tokens,omitempty"`
	}{SessionID: sessionID, RunID: runID, Prompt: text, Attachments: attachments, MaxInputTokens: maxInputTokens})
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

func (c *Client) InitAgent(ctx context.Context, wsID string, interactive bool) error {
	body, _ := json.Marshal(struct {
		Interactive bool `json:"interactive"`
	}{Interactive: interactive})
	path := expandPath(agentInitPath, "id", wsID)
	return c.doJSON(ctx, http.MethodPost, path, bytes.NewReader(body), nil)
}

func (c *Client) RefreshPromptContext(ctx context.Context, wsID string) error {
	path := expandPath(agentRefreshPath, "id", wsID)
	return c.doJSON(ctx, http.MethodPost, path, nil, nil)
}

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

func (c *Client) CancelPrompt(ctx context.Context, wsID, sessionID string) error {
	path := expandPath(cancelPath, "id", wsID, "sid", sessionID)
	return c.doJSON(ctx, http.MethodPost, path, nil, nil)
}

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

func (c *Client) SetPermissionsSkip(ctx context.Context, wsID string, skip bool) error {
	body, _ := json.Marshal(struct {
		Skip bool `json:"skip"`
	}{Skip: skip})
	path := expandPath(permissionSkipPath, "id", wsID)
	return c.doJSON(ctx, http.MethodPost, path, bytes.NewReader(body), nil)
}

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

func requestURL(path string) string {
	return "http://crush" + path
}

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

func IsClientNotAttached(err error) bool {
	return err != nil && strings.Contains(err.Error(), "client not attached")
}

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

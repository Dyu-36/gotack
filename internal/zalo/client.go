// Package zalo connects Gotack to the official Zalo Bot API so the local
// agent can receive requests from and send results to a paired Zalo chat.
package zalo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://bot-api.zaloplatforms.com"

// Client calls the Zalo Bot API on behalf of one bot token.
type Client struct {
	token string
	base  string
	http  *http.Client
}

// NewClient validates token shape and creates a bounded HTTP client.
func NewClient(token string) (*Client, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("Chưa có Bot Token Zalo")
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("ZALO_BOT_API_BASE")), "/")
	if base == "" {
		base = defaultBaseURL
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ForceAttemptHTTP2 = true
	transport.MaxIdleConnsPerHost = 2
	transport.IdleConnTimeout = 15 * time.Second
	return &Client{
		token: token,
		base:  base,
		http: &http.Client{
			Transport: transport,
			Timeout:   120 * time.Second,
		},
	}, nil
}

// BotInfo identifies the bot behind a token.
type BotInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Update is one normalized inbound Bot API event.
type Update struct {
	UpdateID      *int64
	MessageID     string
	ChatID        string
	SenderName    string
	Text          string
	AttachmentURL string
}

// APIError is a structured Bot API failure.
type APIError struct {
	Method      string
	Code        int
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("zalo: %s failed (%d): %s", e.Method, e.Code, e.Description)
}

// GetMe validates the token and returns the bot identity.
func (c *Client) GetMe(ctx context.Context) (BotInfo, error) {
	result, err := c.call(ctx, "getMe", map[string]any{}, 20*time.Second)
	if err != nil {
		return BotInfo{}, err
	}
	name := firstString(result, "display_name", "account_name", "username", "name", "id")
	if name == "" {
		name = "Bot Zalo"
	}
	return BotInfo{ID: firstString(result, "id"), Name: name}, nil
}

// GetUpdates long-polls for all updates returned in one response. It accepts
// both array-shaped and single-update Bot API payloads.
func (c *Client) GetUpdates(ctx context.Context, offset *int64, pollTimeout time.Duration) ([]Update, error) {
	seconds := int(pollTimeout / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	body := map[string]any{"timeout": seconds}
	if offset != nil {
		body["offset"] = *offset
	}
	result, err := c.call(ctx, "getUpdates", body, pollTimeout+20*time.Second)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Code == http.StatusRequestTimeout {
			return nil, nil
		}
		return nil, err
	}
	return parseUpdates(result), nil
}

// SendMessage delivers a text reply. Markdown is attempted first for plain
// text and retried without parse_mode when Zalo rejects it.
func (c *Client) SendMessage(ctx context.Context, chatID, text string) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return errors.New("zalo: chat id is required")
	}
	body := map[string]any{"chat_id": chatID, "text": text}
	if !strings.Contains(text, "http://") && !strings.Contains(text, "https://") {
		markdown := map[string]any{"chat_id": chatID, "text": text, "parse_mode": "markdown"}
		if _, err := c.call(ctx, "sendMessage", markdown, 30*time.Second); err == nil {
			return nil
		}
	}
	_, err := c.call(ctx, "sendMessage", body, 30*time.Second)
	return err
}

// SendChatAction is best effort because some Zalo bot plans do not expose it.
func (c *Client) SendChatAction(ctx context.Context, chatID, action string) {
	_, _ = c.call(ctx, "sendChatAction", map[string]any{"chat_id": chatID, "action": action}, 10*time.Second)
}

// SendPhotoURL sends an uploaded image URL. A caption rejected by Zalo is
// retried as a separate message.
func (c *Client) SendPhotoURL(ctx context.Context, chatID, photoURL, caption string) error {
	body := map[string]any{"chat_id": chatID, "photo": photoURL}
	if strings.TrimSpace(caption) != "" {
		body["caption"] = caption
	}
	if _, err := c.call(ctx, "sendPhoto", body, 30*time.Second); err == nil {
		return nil
	} else if caption == "" {
		return err
	}
	if _, err := c.call(ctx, "sendPhoto", map[string]any{"chat_id": chatID, "photo": photoURL}, 30*time.Second); err != nil {
		return err
	}
	return c.SendMessage(ctx, chatID, caption)
}

// DeleteWebhook makes long polling authoritative for this bot token.
func (c *Client) DeleteWebhook(ctx context.Context) error {
	_, err := c.call(ctx, "deleteWebhook", map[string]any{}, 15*time.Second)
	return err
}

// DownloadAttachment stores one inbound file under dir and enforces the same
// 45 MiB bound used for outbound delivery.
func (c *Client) DownloadAttachment(ctx context.Context, rawURL, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("Không tạo được thư mục nhận tệp: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("Không tạo được yêu cầu tải tệp: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("Không tải được tệp: %s", c.redact(err.Error()))
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("Không tải được tệp (HTTP %d)", resp.StatusCode)
	}
	name := attachmentFileName(rawURL, resp.Header.Get("Content-Type"))
	path := filepath.Join(dir, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("Không lưu được tệp vừa tải: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(resp.Body, maxUploadBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("Không đọc được tệp vừa tải: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("Không đóng được tệp vừa tải: %w", closeErr)
	}
	if written == 0 {
		_ = os.Remove(path)
		return "", errors.New("Tệp nhận được bị rỗng")
	}
	if written > maxUploadBytes {
		_ = os.Remove(path)
		return "", errors.New("Tệp gửi vào quá lớn (giới hạn 45 MB)")
	}
	return path, nil
}

func (c *Client) call(ctx context.Context, method string, body any, timeout time.Duration) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("zalo: encode %s: %w", method, err)
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		requestCtx, cancel := context.WithTimeout(ctx, timeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, c.base+"/bot"+c.token+"/"+method, bytes.NewReader(payload))
		if err != nil {
			cancel()
			return nil, fmt.Errorf("zalo: build %s: %w", method, err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			if attempt == 0 && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF)) {
				time.Sleep(300 * time.Millisecond)
				continue
			}
			break
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		cancel()
		if readErr != nil {
			return nil, fmt.Errorf("zalo: %s: read response: %w", method, readErr)
		}
		var envelope struct {
			OK          *bool           `json:"ok"`
			Result      json.RawMessage `json:"result"`
			ErrorCode   int             `json:"error_code"`
			Description string          `json:"description"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, fmt.Errorf("zalo: %s: decode response: %w", method, err)
		}
		if resp.StatusCode/100 != 2 || (envelope.OK != nil && !*envelope.OK) {
			code := envelope.ErrorCode
			if code == 0 {
				code = resp.StatusCode
			}
			description := c.redact(envelope.Description)
			if description == "" {
				description = http.StatusText(resp.StatusCode)
			}
			return nil, &APIError{Method: method, Code: code, Description: description}
		}
		if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
			return json.RawMessage(`{}`), nil
		}
		return envelope.Result, nil
	}
	return nil, fmt.Errorf("zalo: %s: %s", method, c.redact(lastErr.Error()))
}

func (c *Client) redact(message string) string {
	return strings.ReplaceAll(message, c.token, "***")
}

func parseUpdates(raw json.RawMessage) []Update {
	var values []json.RawMessage
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '[' {
		_ = json.Unmarshal(raw, &values)
	} else {
		var wrapper struct {
			Updates []json.RawMessage `json:"updates"`
		}
		if json.Unmarshal(raw, &wrapper) == nil && len(wrapper.Updates) > 0 {
			values = wrapper.Updates
		} else {
			values = []json.RawMessage{raw}
		}
	}
	updates := make([]Update, 0, len(values))
	for _, value := range values {
		if update, ok := parseUpdate(value); ok {
			updates = append(updates, update)
		}
	}
	return updates
}

func parseUpdate(raw json.RawMessage) (Update, bool) {
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil {
		return Update{}, false
	}
	message := value
	if nested, ok := value["message"].(map[string]any); ok {
		message = nested
	}
	chatID := looseString(message["chat_id"])
	if chat, ok := message["chat"].(map[string]any); ok && chatID == "" {
		chatID = looseString(chat["id"])
	}
	if chatID == "" {
		chatID = looseString(value["chat_id"])
	}
	if chatID == "" {
		return Update{}, false
	}
	var updateID *int64
	if parsed, ok := looseInt64(value["update_id"]); ok {
		updateID = &parsed
	} else if parsed, ok := looseInt64(value["id"]); ok {
		updateID = &parsed
	}
	senderName := "Zalo"
	if from, ok := message["from"].(map[string]any); ok {
		if name := firstMapString(from, "display_name", "name", "username", "id"); name != "" {
			senderName = name
		}
	}
	return Update{
		UpdateID:      updateID,
		MessageID:     firstMapString(message, "message_id", "msg_id", "id"),
		ChatID:        chatID,
		SenderName:    senderName,
		Text:          firstMapString(message, "text", "caption", "message"),
		AttachmentURL: findAttachmentURL(message),
	}, true
}

func findAttachmentURL(value any) string {
	switch node := value.(type) {
	case map[string]any:
		for _, key := range []string{"download_url", "file_url", "attachment_url", "url", "href", "link"} {
			if candidate := strings.TrimSpace(looseString(node[key])); strings.HasPrefix(candidate, "https://") || strings.HasPrefix(candidate, "http://") {
				return candidate
			}
		}
		for _, key := range []string{"attachment", "attachments", "file", "files", "photo", "document", "payload"} {
			if found := findAttachmentURL(node[key]); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range node {
			if found := findAttachmentURL(child); found != "" {
				return found
			}
		}
	}
	return ""
}

func attachmentFileName(rawURL, contentType string) string {
	name := ""
	if parsed, err := url.Parse(rawURL); err == nil {
		name = filepath.Base(parsed.Path)
	}
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\\|?*`, r) {
			return '_'
		}
		return r
	}, name))
	if name == "" || name == "." {
		name = "zalo-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	if filepath.Ext(name) == "" {
		switch {
		case strings.Contains(contentType, "png"):
			name += ".png"
		case strings.Contains(contentType, "jpeg"):
			name += ".jpg"
		case strings.Contains(contentType, "pdf"):
			name += ".pdf"
		default:
			name += ".bin"
		}
	}
	return name
}

func firstString(raw json.RawMessage, keys ...string) string {
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return firstMapString(value, keys...)
}

func firstMapString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := looseString(value[key]); text != "" {
			return text
		}
	}
	return ""
}

func looseString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	default:
		return ""
	}
}

func looseInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

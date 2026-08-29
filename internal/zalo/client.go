// Package zalo connects gotack to the official Zalo Bot API so the local
// agent can receive requests from and send results to a Zalo chat.
//
// The Bot API is a Telegram-style HTTP API: every call is
// POST {base}/bot<token>/<method> and returns the envelope
// {"ok": bool, "result": ..., "error_code": n, "description": "..."}.
// Long-polling getUpdates makes inbound messages work without a public
// webhook endpoint, which is what a desktop app needs.
package zalo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://bot-api.zaloplatforms.com"

// Client calls the Zalo Bot API on behalf of one bot token.
type Client struct {
	token string
	base  string
	http  *http.Client
}

// NewClient returns a client for the given bot token.
func NewClient(token string) *Client {
	return &Client{
		token: token,
		base:  defaultBaseURL,
		http:  &http.Client{Timeout: 60 * time.Second},
	}
}

// BotInfo identifies the bot behind a token; used to validate credentials.
type BotInfo struct {
	ID          string `json:"id"`
	Name        string `json:"account_name"`
	AccountType string `json:"account_type"`
}

// Sender is the author of an inbound message.
type Sender struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// Chat is the conversation a message belongs to.
type Chat struct {
	ID       string `json:"id"`
	ChatType string `json:"chat_type"`
}

// Message is one inbound chat message.
type Message struct {
	MessageID string `json:"message_id"`
	From      Sender `json:"from"`
	Chat      Chat   `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text"`
}

// Update is one inbound event. The Bot API delivers a single update per
// getUpdates call, not an array.
type Update struct {
	EventName string  `json:"event_name"`
	Message   Message `json:"message"`
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
	var info BotInfo
	if err := c.call(ctx, "getMe", nil, &info); err != nil {
		return BotInfo{}, err
	}
	return info, nil
}

// GetUpdates long-polls for the next inbound update. A nil update means the
// poll window elapsed without new messages (or the API answered 408).
func (c *Client) GetUpdates(ctx context.Context, pollTimeout time.Duration) (*Update, error) {
	var update Update
	err := c.call(ctx, "getUpdates", map[string]string{"timeout": fmt.Sprint(int(pollTimeout.Seconds()))}, &update)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == 408 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if update.EventName == "" && update.Message.MessageID == "" {
		return nil, nil
	}
	return &update, nil
}

// SendMessage delivers a text reply to one chat.
func (c *Client) SendMessage(ctx context.Context, chatID, text string) error {
	if chatID == "" {
		return errors.New("zalo: chat id is required")
	}
	return c.call(ctx, "sendMessage", map[string]string{"chat_id": chatID, "text": text}, nil)
}

func (c *Client) call(ctx context.Context, method string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("zalo: encode %s: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/bot"+c.token+"/"+method, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("zalo: build %s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("zalo: %s: %w", method, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("zalo: %s: read response: %w", method, err)
	}

	var envelope struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("zalo: %s: decode response: %w", method, err)
	}
	if !envelope.OK {
		return &APIError{Method: method, Code: envelope.ErrorCode, Description: envelope.Description}
	}
	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return fmt.Errorf("zalo: %s: decode result: %w", method, err)
		}
	}
	return nil
}

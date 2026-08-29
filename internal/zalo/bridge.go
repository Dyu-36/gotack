package zalo

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// maxReplyChars is the Zalo Bot API text limit; replies are truncated to fit.
const maxReplyChars = 2000

// fallback replies are product copy sent to Zalo users when the agent cannot
// serve the request. They are the only user-facing strings in this package.
const (
	replyBusy        = "Bot đang xử lý yêu cầu trước đó, vui lòng đợi chút rồi thử lại."
	replyUnavailable = "Agent chưa sẵn sàng trên máy. Hãy mở Gotack, kết nối workspace rồi thử lại."
	replyFailed      = "Không gửi được yêu cầu tới agent. Vui lòng thử lại."
	replyTruncated   = "…(kết quả dài, xem tiếp trong Gotack)"
)

// Starter submits one inbound request to the agent and returns the session id
// that will carry the answer. Implementations decide which session and
// workspace serve the chat.
type Starter func(ctx context.Context, chatID, senderName, text string) (sessionID string, err error)

// Status is a snapshot of bridge health for the settings UI.
type Status struct {
	Running     bool   `json:"running"`
	BotName     string `json:"bot_name,omitempty"`
	LastError   string `json:"last_error,omitempty"`
	LastChatID  string `json:"last_chat_id,omitempty"`
	LastSender  string `json:"last_sender,omitempty"`
	LastText    string `json:"last_text,omitempty"`
	LastSeenAt  int64  `json:"last_seen_at,omitempty"`
	LastReplyAt int64  `json:"last_reply_at,omitempty"`
}

// Bridge polls the Zalo Bot API, forwards allowed chat messages to the agent
// through Starter, and sends completed agent answers back to the chat.
type Bridge struct {
	client  *Client
	allowed map[string]bool
	start   Starter
	log     *slog.Logger

	mu          sync.Mutex
	sessions    map[string]string // session id -> chat id
	busy        map[string]bool   // chat id -> agent turn in flight
	seen        map[string]bool   // processed message ids
	seenOrder   []string
	status      Status
}

// NewBridge wires a bridge for one bot token. allowed lists the chat ids the
// bridge may serve; an empty list means every chat is ignored.
func NewBridge(token string, allowed []string, start Starter, log *slog.Logger) *Bridge {
	if log == nil {
		log = slog.Default()
	}
	allow := make(map[string]bool, len(allowed))
	for _, id := range allowed {
		if id = strings.TrimSpace(id); id != "" {
			allow[id] = true
		}
	}
	return &Bridge{
		client:   NewClient(token),
		allowed:  allow,
		start:    start,
		log:      log,
		sessions: make(map[string]string),
		busy:     make(map[string]bool),
		seen:     make(map[string]bool),
	}
}

// Run validates the token, then polls until ctx is cancelled. It returns an
// error only when the token itself is rejected; transient poll failures are
// logged and retried with backoff.
func (b *Bridge) Run(ctx context.Context) error {
	info, err := b.client.GetMe(ctx)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.status.BotName = info.Name
	b.status.Running = true
	b.mu.Unlock()
	b.log.Info("zalo bridge connected", "bot", info.Name)

	const pollTimeout = 25 * time.Second
	backoff := time.Second
	for ctx.Err() == nil {
		update, err := b.client.GetUpdates(ctx, pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			b.noteError(err.Error())
			b.log.Warn("zalo poll failed", "err", err)
			select {
			case <-ctx.Done():
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		backoff = time.Second
		if update != nil {
			b.dispatch(ctx, update)
		}
	}

	b.mu.Lock()
	b.status.Running = false
	b.mu.Unlock()
	return ctx.Err()
}

// Done sends the final agent answer for a session back to its chat.
func (b *Bridge) Done(sessionID, text string) {
	b.mu.Lock()
	chatID, ok := b.sessions[sessionID]
	inFlight := b.busy[chatID]
	b.mu.Unlock()
	if !ok || !inFlight {
		return
	}

	answer := strings.TrimSpace(text)
	if answer == "" {
		answer = replyFailed
	}
	if len(answer) > maxReplyChars {
		answer = answer[:maxReplyChars-1] + replyTruncated
	}
	if err := b.client.SendMessage(context.Background(), chatID, answer); err != nil {
		b.noteError(err.Error())
		b.log.Warn("zalo reply failed", "chat", chatID, "err", err)
		return
	}

	b.mu.Lock()
	delete(b.sessions, sessionID)
	delete(b.busy, chatID)
	b.status.LastReplyAt = time.Now().Unix()
	b.mu.Unlock()
}

// Status returns the current bridge snapshot.
func (b *Bridge) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.status
}

func (b *Bridge) dispatch(ctx context.Context, update *Update) {
	msg := update.Message
	text := strings.TrimSpace(msg.Text)
	if msg.MessageID == "" || msg.Chat.ID == "" || text == "" {
		return
	}
	b.rememberMessage(msg.MessageID)

	chatID := msg.Chat.ID
	b.mu.Lock()
	b.status.LastChatID = chatID
	b.status.LastSender = displayName(msg)
	b.status.LastText = text
	b.status.LastSeenAt = time.Now().Unix()
	allowed := b.allowed[chatID]
	busy := b.busy[chatID]
	b.mu.Unlock()
	if !allowed {
		return
	}
	if busy {
		_ = b.client.SendMessage(ctx, chatID, replyBusy)
		return
	}

	sessionID, err := b.start(ctx, chatID, displayName(msg), text)
	if err != nil {
		b.noteError(err.Error())
		b.log.Warn("zalo request rejected", "chat", chatID, "err", err)
		_ = b.client.SendMessage(ctx, chatID, replyUnavailable)
		return
	}

	b.mu.Lock()
	b.sessions[sessionID] = chatID
	b.busy[chatID] = true
	b.mu.Unlock()
}

// rememberMessage keeps a bounded window of processed message ids so a repeat
// delivery of the same update is answered once, regardless of server cursor
// semantics.
func (b *Bridge) rememberMessage(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.seen[id] {
		return
	}
	b.seen[id] = true
	b.seenOrder = append(b.seenOrder, id)
	const window = 256
	if len(b.seenOrder) > window {
		delete(b.seen, b.seenOrder[0])
		b.seenOrder = b.seenOrder[len(b.seenOrder)-window:]
	}
}

func (b *Bridge) noteError(msg string) {
	b.mu.Lock()
	b.status.LastError = msg
	b.mu.Unlock()
}



func displayName(msg Message) string {
	if name := strings.TrimSpace(msg.From.Name); name != "" {
		return name
	}
	return strings.TrimSpace(msg.From.DisplayName)
}

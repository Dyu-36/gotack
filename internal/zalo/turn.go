package zalo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Turn struct {
	ID            string
	WorkspaceID   string
	WorkspacePath string
	SessionID     string
}

type Completion struct {
	RunID     string
	SessionID string
	Text      string
	Error     string
	Cancelled bool
}

type activeTurn struct {
	Turn
	ctx       context.Context
	cancel    context.CancelFunc
	completed bool
}

func (m *Manager) startTurn(ctx context.Context, client *Client, update Update) {
	runtime := m.runtimeSnapshot()
	if runtime.Prepare == nil || runtime.Run == nil {
		m.reply(ctx, client, update.ChatID, "Gotack is not ready to run this request.")
		return
	}
	turnCtx, cancel := context.WithCancel(ctx)
	active := &activeTurn{Turn: Turn{ID: uuid.NewString()}, ctx: turnCtx, cancel: cancel}
	m.mu.Lock()
	if !contains(m.state.PairedChatIDs, update.ChatID) || m.state.Token != client.token {
		m.mu.Unlock()
		cancel()
		return
	}
	if _, busy := m.active[update.ChatID]; busy {
		m.mu.Unlock()
		cancel()
		m.reply(ctx, client, update.ChatID, "Gotack is still handling the previous request. Use /stop to cancel it.")
		return
	}
	existing := m.state.ChatSessions[update.ChatID]
	m.active[update.ChatID] = active
	m.mu.Unlock()
	fail := func(err error) {
		if m.finishTurn(update.ChatID, active.ID) {
			m.log.Warn("Zalo turn failed", "run", active.ID, "err", err)
			if ctx.Err() == nil && m.paired(update.ChatID) {
				m.reply(ctx, client, update.ChatID, "Request failed: "+err.Error())
			}
		}
	}
	run, err := runtime.Prepare(turnCtx, existing, update.ChatID)
	if err != nil {
		fail(err)
		return
	}
	run.ID = active.ID
	if run.SessionID == "" || run.WorkspaceID == "" || !filepath.IsAbs(run.WorkspacePath) {
		fail(errors.New("incomplete workspace/session identity"))
		return
	}
	m.mu.Lock()
	current := m.active[update.ChatID]
	if current != active || turnCtx.Err() != nil || !contains(m.state.PairedChatIDs, update.ChatID) {
		m.mu.Unlock()
		cancel()
		return
	}
	active.Turn = run
	m.state.ChatSessions[update.ChatID] = run.SessionID
	err = m.saveLocked()
	m.mu.Unlock()
	if err != nil {
		fail(err)
		return
	}
	content := strings.TrimSpace(update.Text)
	if update.AttachmentURL != "" {
		inbox := filepath.Join(run.WorkspacePath, ".tack", "zalo-inbox", run.ID)
		if err := os.MkdirAll(inbox, 0o700); err != nil {
			fail(err)
			return
		}
		path, err := client.DownloadAttachment(turnCtx, update.AttachmentURL, inbox)
		if err != nil {
			fail(err)
			return
		}
		if content == "" {
			content = defaultFilePrompt
		}
		content += "\n\nZalo attachment saved at: " + path
	}
	if content == "" {
		fail(errors.New("empty request"))
		return
	}
	if turnCtx.Err() != nil {
		fail(turnCtx.Err())
		return
	}
	client.SendChatAction(turnCtx, update.ChatID, "typing")
	if err := runtime.Run(turnCtx, run, content); err != nil {
		m.mu.Lock()
		completed := active.completed
		m.mu.Unlock()
		if !completed {
			fail(err)
		}
	}
}

func (m *Manager) finishTurn(chatID, runID string) bool {
	m.mu.Lock()
	active := m.active[chatID]
	if active == nil || active.ID != runID {
		m.mu.Unlock()
		return false
	}
	delete(m.active, chatID)
	m.mu.Unlock()
	active.cancel()
	return true
}

func (m *Manager) Done(completion Completion) {
	if completion.RunID == "" || completion.SessionID == "" {
		return
	}
	m.mu.Lock()
	var chatID string
	var active *activeTurn
	for chat, candidate := range m.active {
		if candidate.ID == completion.RunID && candidate.SessionID == completion.SessionID && !candidate.completed {
			chatID, active = chat, candidate
			break
		}
	}
	if active == nil {
		m.mu.Unlock()
		return
	}
	active.completed = true
	token, run := m.state.Token, active.Turn
	m.mu.Unlock()
	go func() {
		defer m.finishTurn(chatID, run.ID)
		if active.ctx.Err() != nil || !m.authorized(chatID, token) {
			return
		}
		client, err := m.newClient(token)
		if err != nil {
			m.setError(err)
			return
		}
		text := completion.Text
		includeFiles := completion.Error == "" && !completion.Cancelled
		if completion.Error != "" {
			text = "Request failed: " + completion.Error + "\n" + text
		}
		if completion.Cancelled {
			text = "Request cancelled.\n" + text
		}
		m.deliverAnswer(active.ctx, client, chatID, text, run.WorkspacePath, includeFiles)
	}()
}

func (m *Manager) authorized(chatID, token string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return token != "" && token == m.state.Token && contains(m.state.PairedChatIDs, chatID)
}

func (m *Manager) deliverAnswer(ctx context.Context, client *Client, chatID, text, workspace string, includeFiles bool) {
	var paths []string
	if includeFiles {
		paths = extractMediaPaths(text, workspace, time.Time{})
	}
	clean := sanitizeReply(text)
	if clean == "" {
		clean = "Done."
	}
	for index, part := range chunkText(clean, maxMessageChars) {
		if ctx.Err() != nil || !m.authorized(chatID, client.token) {
			return
		}
		if index > 0 {
			timer := time.NewTimer(200 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		if err := client.SendMessage(ctx, chatID, part); err != nil {
			m.setError(err)
			return
		}
	}
	for _, path := range paths {
		if ctx.Err() != nil || !m.authorized(chatID, client.token) {
			return
		}
		if err := m.sendPath(ctx, client, chatID, path); err != nil {
			m.reply(ctx, client, chatID, "Could not send "+filepath.Base(path)+": "+err.Error())
		}
	}
}

func (m *Manager) stopTurn(ctx context.Context, client *Client, chatID string) {
	m.mu.Lock()
	active := m.active[chatID]
	var run Turn
	if active != nil {
		run = active.Turn
	}
	m.mu.Unlock()
	if active == nil {
		m.reply(ctx, client, chatID, "No active request.")
		return
	}
	active.cancel()
	runtime := m.runtimeSnapshot()
	if run.SessionID != "" {
		if runtime.Stop == nil {
			m.reply(ctx, client, chatID, "Cannot cancel the engine request.")
			return
		}
		if err := runtime.Stop(ctx, run); err != nil {
			m.reply(ctx, client, chatID, fmt.Sprintf("Cancellation failed: %v", err))
			return
		}
	}
	m.finishTurn(chatID, run.ID)
	m.reply(ctx, client, chatID, "Request cancelled.")
}

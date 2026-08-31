package zalo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	pollTimeout       = 25 * time.Second
	minBackoff        = time.Second
	maxBackoff        = 30 * time.Second
	seenCapacity      = 200
	defaultFilePrompt = "Tôi vừa gửi một tệp qua Zalo. Hãy mở tệp này và xử lý giúp tôi."
)

const helpText = "✅ Đã kết nối với Gotack trên máy của bạn.\n\nNhắn bình thường để nhờ Gotack làm việc. Các lệnh nhanh:\n/screenshot — chụp ảnh màn hình máy tính gửi qua Zalo\n/files — xem danh sách tệp của workspace\n/send <tên file> — gửi ảnh / xlsx / pdf / docx... qua Zalo\n/new — mở hội thoại mới\n/stop — dừng việc đang chạy\n/status — xem trạng thái\n/model — xem mô hình đang dùng\n/help — xem lại danh sách này"

func (m *Manager) run(ctx context.Context) {
	defer func() {
		m.mu.Lock()
		m.running = false
		m.cancel = nil
		m.mu.Unlock()
	}()
	backoff := minBackoff
	var client *Client
	var clientToken string

	for {
		if ctx.Err() != nil {
			return
		}
		state := m.snapshot()
		if state.Token == "" {
			return
		}
		if client == nil || clientToken != state.Token {
			created, err := m.newClient(state.Token)
			if err != nil {
				m.setError(err)
				return
			}
			client, clientToken = created, state.Token
			_ = client.DeleteWebhook(ctx)
		}

		updates, err := client.GetUpdates(ctx, state.UpdateOffset, pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			m.setError(err)
			m.log.Warn("zalo poll failed", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			client = nil
			continue
		}
		backoff = minBackoff
		m.setError(nil)

		for _, update := range updates {
			if update.UpdateID != nil {
				next := *update.UpdateID + 1
				m.mu.Lock()
				if m.state.UpdateOffset == nil || next > *m.state.UpdateOffset {
					m.state.UpdateOffset = &next
					if err := m.saveLocked(); err != nil {
						m.lastError = err.Error()
					}
				}
				m.mu.Unlock()
			}
			if !m.remember(update) {
				continue
			}
			go m.dispatch(context.Background(), client, update)
		}
	}
}

func (m *Manager) remember(update Update) bool {
	key := update.MessageID
	if key == "" && update.UpdateID != nil {
		key = fmt.Sprintf("update:%d", *update.UpdateID)
	}
	if key == "" {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.seen {
		if existing == key {
			return false
		}
	}
	m.seen = append(m.seen, key)
	if len(m.seen) > seenCapacity {
		copy(m.seen, m.seen[len(m.seen)-seenCapacity:])
		m.seen = m.seen[:seenCapacity]
	}
	return true
}

func (m *Manager) dispatch(ctx context.Context, client *Client, update Update) {
	text := strings.TrimSpace(update.Text)
	command := strings.ToLower(strings.Fields(text)[0])
	state := m.snapshot()
	paired := contains(state.PairedChatIDs, update.ChatID)

	if command == "/pair" || strings.HasPrefix(strings.ToLower(text), "/pair") {
		m.handlePair(ctx, client, update.ChatID, text, paired, state.PairingCode)
		return
	}
	if !paired {
		m.log.Info("zalo ignored unpaired chat", "chat", update.ChatID)
		return
	}

	switch command {
	case "/help", "/start":
		m.reply(ctx, client, update.ChatID, helpText)
	case "/screenshot", "/cap", "/screen":
		m.sendScreenshot(ctx, client, update.ChatID)
	case "/send", "/file", "/files", "/guifile":
		argument := ""
		if _, tail, ok := strings.Cut(text, " "); ok {
			argument = strings.TrimSpace(tail)
		}
		m.handleSendFile(ctx, client, update.ChatID, argument)
	case "/new":
		m.mu.Lock()
		delete(m.state.ChatSessions, update.ChatID)
		err := m.saveLocked()
		m.mu.Unlock()
		if err != nil {
			m.reply(ctx, client, update.ChatID, "⚠️ "+err.Error())
		} else {
			m.reply(ctx, client, update.ChatID, "🆕 Đã mở hội thoại mới. Nhắn việc cần làm nhé.")
		}
	case "/stop":
		m.stopTurn(ctx, client, update.ChatID)
	case "/status":
		m.sendRuntimeStatus(ctx, client, update.ChatID)
	case "/model":
		m.sendModel(ctx, client, update.ChatID)
	default:
		m.startTurn(ctx, client, update)
	}
}

func (m *Manager) handlePair(ctx context.Context, client *Client, chatID, text string, paired bool, expected string) {
	if paired {
		m.reply(ctx, client, chatID, "✅ Tài khoản Zalo này đã ghép cặp với Gotack rồi.\n\n/help để xem các lệnh.")
		return
	}
	fields := strings.Fields(text)
	code := ""
	if len(fields) > 1 {
		code = fields[1]
	}
	if expected == "" {
		m.reply(ctx, client, chatID, "⚠️ Chưa có mã ghép cặp. Hãy mở Gotack → Cài đặt → Zalo để lấy mã.")
		return
	}
	if code != expected {
		m.reply(ctx, client, chatID, "❌ Mã ghép cặp không đúng. Hãy xem mã 6 số trong Cài đặt → Zalo và nhắn lại /pair <mã>.")
		return
	}
	m.mu.Lock()
	if !contains(m.state.PairedChatIDs, chatID) {
		m.state.PairedChatIDs = append(m.state.PairedChatIDs, chatID)
	}
	m.state.PairingCode = pairingCode()
	err := m.saveLocked()
	m.mu.Unlock()
	if err != nil {
		m.reply(ctx, client, chatID, "⚠️ "+err.Error())
		return
	}
	m.log.Info("zalo chat paired", "chat", chatID)
	m.reply(ctx, client, chatID, helpText)
}

func (m *Manager) startTurn(ctx context.Context, client *Client, update Update) {
	if m.runtime.Start == nil {
		m.reply(ctx, client, update.ChatID, "Gotack hiện chưa sẵn sàng xử lý yêu cầu.")
		return
	}
	m.mu.Lock()
	if _, busy := m.active[update.ChatID]; busy {
		m.mu.Unlock()
		m.reply(ctx, client, update.ChatID, "Bot đang xử lý yêu cầu trước đó, vui lòng đợi chút rồi thử lại.")
		return
	}
	existingSession := m.state.ChatSessions[update.ChatID]
	m.active[update.ChatID] = "starting"
	m.mu.Unlock()

	content := strings.TrimSpace(update.Text)
	if update.AttachmentURL != "" {
		client.SendChatAction(ctx, update.ChatID, "typing")
		inbox := filepath.Join(os.TempDir(), "gotack-zalo-inbox")
		path, err := client.DownloadAttachment(ctx, update.AttachmentURL, inbox)
		if err != nil {
			m.reply(ctx, client, update.ChatID, "⚠️ Không tải được tệp bạn gửi: "+err.Error())
		} else {
			m.reply(ctx, client, update.ChatID, "📥 Đã nhận "+filepath.Base(path)+", đang xử lý...")
			if content == "" {
				content = defaultFilePrompt
			}
			content += "\n\nTệp Zalo đã tải về máy tại: " + path
		}
	}
	if content == "" {
		m.mu.Lock()
		delete(m.active, update.ChatID)
		m.mu.Unlock()
		return
	}
	startedAt := time.Now().Add(-5 * time.Second)
	sessionID, err := m.runtime.Start(ctx, existingSession, update.ChatID, content)
	if err != nil {
		m.mu.Lock()
		delete(m.active, update.ChatID)
		m.mu.Unlock()
		m.log.Warn("zalo request rejected", "chat", update.ChatID, "err", err)
		m.reply(ctx, client, update.ChatID, "Gotack hiện chưa sẵn sàng xử lý yêu cầu.")
		return
	}
	m.mu.Lock()
	m.active[update.ChatID] = sessionID
	m.state.ChatSessions[update.ChatID] = sessionID
	if err := m.saveLocked(); err != nil {
		m.lastError = err.Error()
	}
	m.mu.Unlock()
	client.SendChatAction(ctx, update.ChatID, "typing")
	m.log.Info("zalo turn started", "chat", update.ChatID, "session", sessionID, "started", startedAt)
}

// Done routes one completed agent answer to the chat that started its session.
// Sessions not active through Zalo are ignored.
func (m *Manager) Done(sessionID, text string) {
	m.mu.Lock()
	chatID := ""
	for chat, activeSession := range m.active {
		if activeSession == sessionID {
			chatID = chat
			delete(m.active, chat)
			break
		}
	}
	state := m.state
	m.mu.Unlock()
	if chatID == "" || state.Token == "" {
		return
	}
	go func() {
		client, err := m.newClient(state.Token)
		if err != nil {
			m.setError(err)
			return
		}
		m.deliverAnswer(context.Background(), client, chatID, text)
	}()
}

func (m *Manager) deliverAnswer(ctx context.Context, client *Client, chatID, text string) {
	workspace := ""
	if m.runtime.Workspace != nil {
		workspace = m.runtime.Workspace()
	}
	paths := extractMediaPaths(text, workspace, time.Time{})
	clean := sanitizeReply(text)
	if clean == "" {
		clean = "✅ Đã xong."
	}
	for _, part := range chunkText(clean, maxMessageChars) {
		if err := client.SendMessage(ctx, chatID, part); err != nil {
			m.setError(err)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	for _, path := range paths {
		if err := m.sendPath(ctx, client, chatID, path); err != nil {
			m.reply(ctx, client, chatID, "⚠️ Không gửi được "+filepath.Base(path)+": "+err.Error())
		}
	}
}

func (m *Manager) stopTurn(ctx context.Context, client *Client, chatID string) {
	m.mu.Lock()
	sessionID, active := m.active[chatID]
	m.mu.Unlock()
	if !active || sessionID == "starting" {
		m.reply(ctx, client, chatID, "Hiện không có việc nào đang chạy.")
		return
	}
	if m.runtime.Stop == nil {
		m.reply(ctx, client, chatID, "⚠️ Không dừng được yêu cầu này.")
		return
	}
	if err := m.runtime.Stop(ctx, sessionID); err != nil {
		m.reply(ctx, client, chatID, "⚠️ Không dừng được: "+err.Error())
		return
	}
	m.reply(ctx, client, chatID, "🛑 Đã dừng việc đang chạy.")
}

func (m *Manager) sendRuntimeStatus(ctx context.Context, client *Client, chatID string) {
	m.mu.Lock()
	sessionID := m.state.ChatSessions[chatID]
	_, busy := m.active[chatID]
	m.mu.Unlock()
	title := "chưa có (nhắn một câu là tạo)"
	if sessionID != "" && m.runtime.Session != nil {
		if value, err := m.runtime.Session(ctx, sessionID); err == nil && value != "" {
			title = value
		}
	}
	activity := "rảnh"
	if busy {
		activity = "đang xử lý một việc"
	}
	m.reply(ctx, client, chatID, fmt.Sprintf("🖥️ Gotack đang chạy trên máy.\nHội thoại: %s\nTrạng thái: %s", title, activity))
}

func (m *Manager) sendModel(ctx context.Context, client *Client, chatID string) {
	if m.runtime.Model == nil {
		m.reply(ctx, client, chatID, "⚠️ Không đọc được mô hình đang dùng.")
		return
	}
	model, err := m.runtime.Model(ctx)
	if err != nil {
		m.reply(ctx, client, chatID, "⚠️ Không đọc được cài đặt: "+err.Error())
		return
	}
	m.reply(ctx, client, chatID, "🧩 Mô hình đang dùng: "+model+"\n(Đổi mô hình trong app Gotack.)")
}

func (m *Manager) sendScreenshot(ctx context.Context, client *Client, chatID string) {
	client.SendChatAction(ctx, chatID, "upload_photo")
	path := filepath.Join(os.TempDir(), fmt.Sprintf("gotack-screen-%d.png", time.Now().UnixNano()))
	if err := captureScreenshot(ctx, path); err != nil {
		m.reply(ctx, client, chatID, "⚠️ Không chụp được màn hình: "+err.Error())
		return
	}
	defer os.Remove(path)
	if err := m.sendPath(ctx, client, chatID, path); err != nil {
		m.reply(ctx, client, chatID, "⚠️ Không gửi được ảnh qua Zalo: "+err.Error())
	}
}

func (m *Manager) handleSendFile(ctx context.Context, client *Client, chatID, argument string) {
	workspace := ""
	if m.runtime.Workspace != nil {
		workspace = m.runtime.Workspace()
	}
	if strings.TrimSpace(argument) == "" {
		files := listOutputFiles(workspace)
		if len(files) == 0 {
			m.reply(ctx, client, chatID, "📂 Workspace này chưa có tệp nào trong thư mục output.\nCách dùng: /send <tên file> hoặc /send <đường dẫn đầy đủ>.")
			return
		}
		lines := make([]string, 0, len(files))
		for _, path := range files {
			lines = append(lines, "• "+filepath.Base(path))
			if len(lines) == 20 {
				break
			}
		}
		m.reply(ctx, client, chatID, "📂 Các tệp có thể gửi:\n"+strings.Join(lines, "\n")+"\n\nGõ: /send <tên file>")
		return
	}
	path := resolveOutboundFile(workspace, argument)
	if path == "" {
		m.reply(ctx, client, chatID, "⚠️ Không tìm thấy tệp \""+strings.TrimSpace(argument)+"\". Gõ /files để xem danh sách.")
		return
	}
	if err := m.sendPath(ctx, client, chatID, path); err != nil {
		m.reply(ctx, client, chatID, "⚠️ Không gửi được "+filepath.Base(path)+": "+err.Error())
	}
}

// SendFile sends one local file to a paired chat, or every paired chat when
// chatID is empty.
func (m *Manager) SendFile(ctx context.Context, path, chatID string) (string, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil || !isSendableFile(absolute) {
		//lint:ignore ST1005 user-facing Vietnamese sentence keeps its capital.
		return "", fmt.Errorf("Không tìm thấy tệp gửi được: %s", path)
	}
	state := m.snapshot()
	if state.Token == "" {
		//lint:ignore ST1005 user-facing Vietnamese sentence keeps its capital.
		return "", fmt.Errorf("Chưa lưu Bot Token Zalo trong Cài đặt")
	}
	targets := state.PairedChatIDs
	if strings.TrimSpace(chatID) != "" {
		if !contains(targets, chatID) {
			//lint:ignore ST1005 user-facing Vietnamese sentence keeps its capital.
			return "", fmt.Errorf("Chat Zalo %s chưa ghép cặp với Gotack", chatID)
		}
		targets = []string{chatID}
	}
	if len(targets) == 0 {
		//lint:ignore ST1005 user-facing Vietnamese sentence keeps its capital.
		return "", fmt.Errorf("Chưa có tài khoản Zalo nào ghép cặp")
	}
	client, err := m.newClient(state.Token)
	if err != nil {
		return "", err
	}
	sent := 0
	failures := make([]string, 0)
	for _, target := range targets {
		if err := m.sendPath(ctx, client, target, absolute); err != nil {
			failures = append(failures, err.Error())
		} else {
			sent++
		}
	}
	if sent == 0 {
		//lint:ignore ST1005 user-facing Vietnamese sentence keeps its capital.
		return "", fmt.Errorf("Không gửi được %s qua Zalo: %s", filepath.Base(absolute), strings.Join(failures, "; "))
	}
	return fmt.Sprintf("Đã gửi %s qua Zalo (%d hội thoại)", filepath.Base(absolute), sent), nil
}

func (m *Manager) sendPath(ctx context.Context, client *Client, chatID, path string) error {
	client.SendChatAction(ctx, chatID, map[bool]string{true: "upload_photo", false: "typing"}[isImageFile(path)])
	url, err := uploadFile(ctx, path)
	if err != nil {
		return err
	}
	if isImageFile(path) {
		return client.SendPhotoURL(ctx, chatID, url, "🖼️ "+filepath.Base(path))
	}
	return client.SendMessage(ctx, chatID, "📎 "+filepath.Base(path)+"\n"+url)
}

func (m *Manager) reply(ctx context.Context, client *Client, chatID, text string) {
	if err := client.SendMessage(ctx, chatID, text); err != nil {
		m.setError(err)
		m.log.Warn("zalo reply failed", "chat", chatID, "err", err)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

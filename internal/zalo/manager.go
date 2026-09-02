package zalo

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Runtime struct {
	Start     func(context.Context, string, string, string) (string, error)
	Stop      func(context.Context, string) error
	Session   func(context.Context, string) (string, error)
	Model     func(context.Context) (string, error)
	Workspace func() string
}

type StoredChannel struct {
	Token         string            `json:"token,omitempty"`
	BotName       string            `json:"bot_name,omitempty"`
	PairingCode   string            `json:"pairing_code,omitempty"`
	PairedChatIDs []string          `json:"paired_chat_ids,omitempty"`
	UpdateOffset  *int64            `json:"update_offset,omitempty"`
	ChatSessions  map[string]string `json:"chat_sessions,omitempty"`
}

type Status struct {
	Configured    bool     `json:"configured"`
	Running       bool     `json:"running"`
	BotName       string   `json:"bot_name,omitempty"`
	TokenSuffix   string   `json:"token_suffix,omitempty"`
	PairingCode   string   `json:"pairing_code,omitempty"`
	PairedChatIDs []string `json:"paired_chat_ids"`
	LastError     string   `json:"last_error,omitempty"`
}

type Manager struct {
	clientFactory func(string) (*Client, error)
	path          string
	runtime       Runtime
	log           *slog.Logger

	state     StoredChannel
	mu        sync.Mutex
	running   bool
	lastError string
	cancel    context.CancelFunc
	active    map[string]string
	seen      []string
}

func NewManager(path string, runtime Runtime, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	m := &Manager{path: path, runtime: runtime, log: log, active: make(map[string]string), clientFactory: NewClient}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &m.state); err != nil {
			m.lastError = "cannot parse saved Zalo channel: " + err.Error()
		}
	} else if !errors.Is(err, os.ErrNotExist) {

		m.lastError = "cannot read saved Zalo channel: " + err.Error()
	}
	m.normalizeLocked()
	return m
}

func (m *Manager) newClient(token string) (*Client, error) {
	if m.clientFactory != nil {
		return m.clientFactory(token)
	}
	return NewClient(token)
}

func (m *Manager) normalizeLocked() {
	if m.state.ChatSessions == nil {
		m.state.ChatSessions = make(map[string]string)
	}
	m.state.Token = strings.TrimSpace(m.state.Token)
	m.state.PairingCode = strings.TrimSpace(m.state.PairingCode)
	if m.state.Token != "" && m.state.PairingCode == "" {
		m.state.PairingCode = pairingCode()
	}
	m.state.PairedChatIDs = uniqueStrings(m.state.PairedChatIDs)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func pairingCode() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return fmt.Sprintf("%06d", binary.LittleEndian.Uint64(raw[:])%1_000_000)
	}
	return "000000"
}

func (m *Manager) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return fmt.Errorf("create Zalo settings directory: %w", err)
	}
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Zalo settings: %w", err)
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write Zalo settings: %w", err)
	}
	if err := os.Rename(tmp, m.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace Zalo settings: %w", err)
	}
	return nil
}

// ImportLegacy migrates the former allow-list configuration once. Existing
// channel state always wins.
//
// Deprecated: this exists only to migrate the deprecated
// appconfig.ZaloSettings.Token and appconfig.ZaloSettings.AllowedChats keys
// (removal target: Gotack v1.0). Its behavior stays unchanged until the
// fields are dropped; then remove this method and its call site together.
func (m *Manager) ImportLegacy(token string, allowed []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Token != "" || strings.TrimSpace(token) == "" {
		return nil
	}
	m.state.Token = strings.TrimSpace(token)
	m.state.PairingCode = pairingCode()
	m.state.PairedChatIDs = uniqueStrings(allowed)
	m.normalizeLocked()
	return m.saveLocked()
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	suffix := ""
	if token := []rune(m.state.Token); len(token) > 0 {
		start := len(token) - 4
		if start < 0 {
			start = 0
		}
		suffix = string(token[start:])
	}
	return Status{
		Configured:    m.state.Token != "",
		Running:       m.running,
		BotName:       m.state.BotName,
		TokenSuffix:   suffix,
		PairingCode:   m.state.PairingCode,
		PairedChatIDs: append([]string{}, m.state.PairedChatIDs...),
		LastError:     m.lastError,
	}
}

func (m *Manager) snapshot() StoredChannel {
	m.mu.Lock()
	defer m.mu.Unlock()
	copyState := m.state
	copyState.PairedChatIDs = append([]string(nil), m.state.PairedChatIDs...)
	copyState.ChatSessions = make(map[string]string, len(m.state.ChatSessions))
	for chatID, sessionID := range m.state.ChatSessions {
		copyState.ChatSessions[chatID] = sessionID
	}
	return copyState
}

func (m *Manager) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err == nil {
		m.lastError = ""
	} else {
		m.lastError = err.Error()
	}
}

func (m *Manager) SetToken(ctx context.Context, token string) (Status, error) {
	token = strings.TrimSpace(token)
	if token == "" {

		return Status{}, errors.New("Bot Token Zalo không được để trống")
	}
	client, err := m.newClient(token)
	if err != nil {
		return Status{}, err
	}
	bot, err := client.GetMe(ctx)
	if err != nil {
		m.setError(err)
		return Status{}, err
	}
	_ = client.DeleteWebhook(ctx)

	m.Stop()
	m.mu.Lock()
	m.state.Token = token
	m.state.BotName = bot.Name
	m.state.UpdateOffset = nil
	if m.state.PairingCode == "" {
		m.state.PairingCode = pairingCode()
	}
	err = m.saveLocked()
	m.lastError = ""
	m.mu.Unlock()
	if err != nil {
		return Status{}, err
	}
	m.Start()
	return m.Status(), nil
}

func (m *Manager) TestConnection(ctx context.Context) (Status, error) {
	state := m.snapshot()
	if state.Token == "" {

		return Status{}, errors.New("Chưa lưu Bot Token Zalo")
	}
	client, err := m.newClient(state.Token)
	if err != nil {
		return Status{}, err
	}
	bot, err := client.GetMe(ctx)
	if err != nil {
		m.setError(err)
		return Status{}, err
	}
	_ = client.DeleteWebhook(ctx)
	m.mu.Lock()
	m.state.BotName = bot.Name
	err = m.saveLocked()
	m.lastError = ""
	m.mu.Unlock()
	if err != nil {
		return Status{}, err
	}
	m.Start()
	return m.Status(), nil
}

func (m *Manager) RemoveToken() (Status, error) {
	m.Stop()
	m.mu.Lock()
	m.state = StoredChannel{ChatSessions: make(map[string]string)}
	m.active = make(map[string]string)
	m.lastError = ""
	err := m.saveLocked()
	m.mu.Unlock()
	return m.Status(), err
}

func (m *Manager) RegeneratePairingCode() (Status, error) {
	m.mu.Lock()
	m.state.PairingCode = pairingCode()
	err := m.saveLocked()
	m.mu.Unlock()
	return m.Status(), err
}

func (m *Manager) Unpair(chatID string) (Status, error) {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {

		return Status{}, errors.New("Thiếu chat_id cần hủy ghép cặp")
	}
	m.mu.Lock()
	out := m.state.PairedChatIDs[:0]
	for _, id := range m.state.PairedChatIDs {
		if id != chatID {
			out = append(out, id)
		}
	}
	m.state.PairedChatIDs = out
	delete(m.state.ChatSessions, chatID)
	delete(m.active, chatID)
	err := m.saveLocked()
	m.mu.Unlock()
	return m.Status(), err
}

func (m *Manager) Start() {
	m.mu.Lock()
	if m.running || m.state.Token == "" {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.running = true
	m.mu.Unlock()
	go m.run(ctx)
}

func (m *Manager) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.running = false
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Manager) SetRuntime(runtime Runtime) {
	m.mu.Lock()
	m.runtime = runtime
	m.mu.Unlock()
}

func (m *Manager) ResetSessions() error {
	m.mu.Lock()
	m.state.ChatSessions = make(map[string]string)
	m.active = make(map[string]string)
	err := m.saveLocked()
	m.mu.Unlock()
	return err
}

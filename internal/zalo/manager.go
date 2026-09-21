package zalo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Runtime struct {
	Prepare   func(context.Context, string, string) (Turn, error)
	Run       func(context.Context, Turn, string) error
	Stop      func(context.Context, Turn) error
	Session   func(context.Context, string) (string, error)
	Model     func(context.Context) (string, error)
	Workspace func() string
}

type StoredChannel struct {
	PairingExpiresAt int64             `json:"pairing_expires_at,omitempty"`
	Token            string            `json:"token,omitempty"`
	BotName          string            `json:"bot_name,omitempty"`
	PairingCode      string            `json:"pairing_code,omitempty"`
	PairedChatIDs    []string          `json:"paired_chat_ids,omitempty"`
	UpdateOffset     *int64            `json:"update_offset,omitempty"`
	ChatSessions     map[string]string `json:"chat_sessions,omitempty"`
}

type Status struct {
	Configured       bool     `json:"configured"`
	Running          bool     `json:"running"`
	BotName          string   `json:"bot_name,omitempty"`
	TokenSuffix      string   `json:"token_suffix,omitempty"`
	PairingCode      string   `json:"pairing_code,omitempty"`
	PairingExpiresAt int64    `json:"pairing_expires_at,omitempty"`
	PairedChatIDs    []string `json:"paired_chat_ids"`
	LastError        string   `json:"last_error,omitempty"`
}

type Manager struct {
	pairAttempts  map[string]pairAttempt
	pairWindow    time.Time
	pairCount     int
	clientFactory func(string) (*Client, error)
	path          string
	runtime       Runtime
	log           *slog.Logger
	lifecycle     sync.Mutex

	state      StoredChannel
	mu         sync.Mutex
	running    bool
	lastError  string
	cancel     context.CancelFunc
	generation uint64
	done       chan struct{}
	active     map[string]*activeTurn
	seen       []string
}

func NewManager(path string, runtime Runtime, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	m := &Manager{path: path, runtime: runtime, log: log, active: make(map[string]*activeTurn), clientFactory: NewClient}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &m.state); err != nil {
			m.state = StoredChannel{}
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
	if m.state.Token != "" && (m.state.PairingCode == "" || m.state.PairingExpiresAt <= time.Now().Unix()) {
		m.rotatePairingLocked(time.Now())
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

func (m *Manager) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return fmt.Errorf("create Zalo settings directory: %w", err)
	}
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Zalo settings: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(m.path), ".zalo-*")
	if err != nil {
		return fmt.Errorf("create Zalo settings transaction: %w", err)
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("write Zalo settings: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close Zalo settings: %w", closeErr)
	}
	if err := os.Rename(name, m.path); err != nil {
		return fmt.Errorf("replace Zalo settings: %w", err)
	}
	return nil
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
		Configured:       m.state.Token != "",
		Running:          m.running,
		BotName:          m.state.BotName,
		TokenSuffix:      suffix,
		PairingCode:      m.state.PairingCode,
		PairingExpiresAt: m.state.PairingExpiresAt,
		PairedChatIDs:    append([]string{}, m.state.PairedChatIDs...),
		LastError:        m.lastError,
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
	if m.state.UpdateOffset != nil {
		offset := *m.state.UpdateOffset
		copyState.UpdateOffset = &offset
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

	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.stopLocked()
	m.mu.Lock()
	previous := m.state
	if previous.Token != token {
		m.state = StoredChannel{ChatSessions: make(map[string]string)}
		m.seen = nil
		m.pairAttempts = nil
		m.pairWindow = time.Time{}
		m.pairCount = 0
	}
	m.state.Token = token
	m.state.BotName = bot.Name
	if m.state.PairingCode == "" || m.state.PairingExpiresAt <= time.Now().Unix() {
		m.rotatePairingLocked(time.Now())
	}
	err = m.saveLocked()
	if err != nil {
		m.state = previous
		m.lastError = err.Error()
	} else {
		m.lastError = ""
	}
	m.mu.Unlock()
	if err != nil {
		return m.Status(), err
	}
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
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.mu.Lock()
	if m.state.Token != state.Token {
		m.mu.Unlock()
		return m.Status(), errors.New("Zalo token changed during connection test")
	}
	m.state.BotName = bot.Name
	err = m.saveLocked()
	m.mu.Unlock()
	m.setError(err)
	if err != nil {
		return m.Status(), err
	}
	return m.Status(), nil
}

func (m *Manager) RemoveToken() (Status, error) {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.stopLocked()
	m.mu.Lock()
	m.state = StoredChannel{ChatSessions: make(map[string]string)}
	m.seen = nil
	m.pairAttempts = nil
	m.lastError = ""
	err := m.saveLocked()
	m.mu.Unlock()
	m.setError(err)
	return m.Status(), err
}

func (m *Manager) RegeneratePairingCode() (Status, error) {
	m.mu.Lock()
	m.rotatePairingLocked(time.Now())
	var err error
	if m.state.PairingCode == "" {
		err = errors.New("cannot obtain secure randomness for Zalo pairing")
	} else {
		err = m.saveLocked()
	}
	m.mu.Unlock()
	m.setError(err)
	return m.Status(), err
}

func (m *Manager) Unpair(chatID string) (Status, error) {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return Status{}, errors.New("missing chat ID")
	}
	m.mu.Lock()
	kept := make([]string, 0, len(m.state.PairedChatIDs))
	for _, id := range m.state.PairedChatIDs {
		if id != chatID {
			kept = append(kept, id)
		}
	}
	m.state.PairedChatIDs = kept
	active := m.active[chatID]
	var run Turn
	if active != nil {
		run = active.Turn
	}
	stop := m.runtime.Stop
	delete(m.state.ChatSessions, chatID)
	delete(m.active, chatID)
	err := m.saveLocked()
	m.mu.Unlock()
	if active != nil {
		active.cancel()
	}
	err = errors.Join(err, cancelRemoteRuns(stop, []Turn{run}))
	m.setError(err)
	return m.Status(), err
}

func (m *Manager) Start() {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.startLocked()
}

func (m *Manager) startLocked() {
	m.mu.Lock()
	if m.running || m.state.Token == "" {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.running = true
	m.generation++
	generation := m.generation
	done := make(chan struct{})
	m.done = done
	m.mu.Unlock()
	go m.run(ctx, generation, done)
}

func (m *Manager) Stop() {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.stopLocked()
}

func (m *Manager) stopLocked() {
	m.mu.Lock()
	cancel, done, stop := m.cancel, m.done, m.runtime.Stop
	runs := make([]Turn, 0, len(m.active))
	cancellations := make([]context.CancelFunc, 0, len(m.active))
	for _, active := range m.active {
		runs = append(runs, active.Turn)
		cancellations = append(cancellations, active.cancel)
	}
	m.active = make(map[string]*activeTurn)
	m.cancel, m.done, m.running = nil, nil, false
	m.generation++
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	for _, stopContext := range cancellations {
		stopContext()
	}
	if err := cancelRemoteRuns(stop, runs); err != nil {
		m.setError(err)
		m.log.Warn("Zalo remote run cancellation failed", "err", err)
	}
	if done != nil {
		<-done
	}
}

func cancelRemoteRuns(stop func(context.Context, Turn) error, runs []Turn) error {
	if stop == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	seen := make(map[string]bool, len(runs))
	var failures []error
	for _, run := range runs {
		key := run.WorkspaceID + "\x00" + run.SessionID
		if run.SessionID == "" || run.WorkspaceID == "" || seen[key] {
			continue
		}
		seen[key] = true
		if err := stop(ctx, run); err != nil {
			failures = append(failures, fmt.Errorf("cancel remote session %s: %w", run.SessionID, err))
		}
	}
	return errors.Join(failures...)
}

func (m *Manager) SetRuntime(runtime Runtime) {
	m.mu.Lock()
	m.runtime = runtime
	m.mu.Unlock()
}

func (m *Manager) runtimeSnapshot() Runtime {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runtime
}

func (m *Manager) ResetSessions() error {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.mu.Lock()
	wasRunning := m.running
	m.mu.Unlock()
	m.stopLocked()
	m.mu.Lock()
	m.state.ChatSessions = make(map[string]string)
	err := m.saveLocked()
	m.mu.Unlock()
	if wasRunning && err == nil {
		m.startLocked()
	}
	return err
}

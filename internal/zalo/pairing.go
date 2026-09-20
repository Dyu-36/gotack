package zalo

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const pairingLifetime = 10 * time.Minute
const pairingAttemptWindow = time.Minute

type pairAttempt struct {
	Since time.Time
	Count int
}

func pairingCode() string {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%06d", value.Int64())
}

func (m *Manager) rotatePairingLocked(now time.Time) {
	previous := m.state.PairingCode
	next := pairingCode()
	for next != "" && next == previous {
		next = pairingCode()
	}
	m.state.PairingCode = next
	m.state.PairingExpiresAt = 0
	if m.state.PairingCode != "" {
		m.state.PairingExpiresAt = now.Add(pairingLifetime).Unix()
	} else {
		m.lastError = "Cannot obtain secure randomness for Zalo pairing"
	}
}

func (m *Manager) acceptPairing(chatID, code string, now time.Time) error {
	if strings.TrimSpace(chatID) == "" || len(chatID) > 256 {
		return errors.New("invalid chat identifier")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if contains(m.state.PairedChatIDs, chatID) {
		return nil
	}
	if m.pairAttempts == nil {
		m.pairAttempts = make(map[string]pairAttempt)
	}
	if now.Sub(m.pairWindow) >= pairingAttemptWindow {
		m.pairWindow, m.pairCount = now, 0
		for id, attempt := range m.pairAttempts {
			if now.Sub(attempt.Since) >= pairingAttemptWindow {
				delete(m.pairAttempts, id)
			}
		}
	}
	attempt := m.pairAttempts[chatID]
	if now.Sub(attempt.Since) >= pairingAttemptWindow {
		attempt = pairAttempt{Since: now}
	}
	if m.pairCount >= 30 || attempt.Count >= 5 || len(m.pairAttempts) >= 1024 {
		return errors.New("too many pairing attempts; retry after one minute")
	}
	attempt.Count++
	m.pairAttempts[chatID] = attempt
	m.pairCount++
	if m.state.PairingCode == "" || now.Unix() >= m.state.PairingExpiresAt {
		return errors.New("pairing code expired; generate a new code in desktop settings")
	}
	if len(code) != 6 || subtle.ConstantTimeCompare([]byte(code), []byte(m.state.PairingCode)) != 1 {
		return errors.New("incorrect pairing code")
	}
	previous := m.state
	m.state.PairedChatIDs = append(append([]string(nil), previous.PairedChatIDs...), chatID)
	m.rotatePairingLocked(now)
	if err := m.saveLocked(); err != nil {
		m.state = previous
		return fmt.Errorf("persist pairing: %w", err)
	}
	delete(m.pairAttempts, chatID)
	return nil
}

func (m *Manager) paired(chatID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return contains(m.state.PairedChatIDs, chatID)
}

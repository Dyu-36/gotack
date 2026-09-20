package zalo

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPairingIsExpiringSingleUseAndDurable(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "zalo.json"), Runtime{}, nil)
	now := time.Now()
	m.state.Token = "test-token"
	m.rotatePairingLocked(now)
	code := m.state.PairingCode
	if len(code) != 6 {
		t.Fatal("missing random code")
	}
	if err := m.acceptPairing("first", code, now); err != nil {
		t.Fatal(err)
	}
	if !m.paired("first") {
		t.Fatal("pairing was not applied")
	}
	saved := NewManager(m.path, Runtime{}, nil)
	if !saved.paired("first") {
		t.Fatal("pairing was not persisted")
	}
	if err := m.acceptPairing("second", code, now); err == nil {
		t.Fatal("consumed code was reusable")
	}
	expired := m.state.PairingCode
	if err := m.acceptPairing("third", expired, now.Add(pairingLifetime)); err == nil {
		t.Fatal("expired code accepted")
	}
}

func TestPairingAttemptsAreBounded(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "zalo.json"), Runtime{}, nil)
	now := time.Now()
	m.rotatePairingLocked(now)
	for range 5 {
		if err := m.acceptPairing("chat", "invalid", now); err == nil {
			t.Fatal("incorrect code accepted")
		}
	}
	if err := m.acceptPairing("chat", m.state.PairingCode, now); err == nil {
		t.Fatal("attempt limit bypassed")
	}
	if err := m.acceptPairing("chat", m.state.PairingCode, now.Add(pairingAttemptWindow)); err != nil {
		t.Fatal(err)
	}
}

func TestPairingPersistenceFailureDoesNotGrantAccess(t *testing.T) {
	m := NewManager(t.TempDir(), Runtime{}, nil)
	now := time.Now()
	m.rotatePairingLocked(now)
	code := m.state.PairingCode
	if err := m.acceptPairing("chat", code, now); err == nil {
		t.Fatal("persisting over directory should fail")
	}
	if m.paired("chat") || m.state.PairingCode != code {
		t.Fatal("failed transaction changed access state")
	}
}

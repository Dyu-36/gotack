package zalo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCompletionMessagePreservesPrefixes(t *testing.T) {
	tests := []struct {
		name       string
		completion Completion
		wantText   string
		wantFiles  bool
	}{
		{
			name:       "success",
			completion: Completion{Text: "done"},
			wantText:   "done",
			wantFiles:  true,
		},
		{
			name:       "error",
			completion: Completion{Text: "details", Error: "provider unavailable"},
			wantText:   "Request failed: provider unavailable\ndetails",
		},
		{
			name:       "cancelled",
			completion: Completion{Text: "partial", Cancelled: true},
			wantText:   "Request cancelled.\npartial",
		},
		{
			name:       "error and cancelled",
			completion: Completion{Text: "partial", Error: "provider unavailable", Cancelled: true},
			wantText:   "Request cancelled.\nRequest failed: provider unavailable\npartial",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotFiles := completionMessage(tt.completion)
			if gotText != tt.wantText || gotFiles != tt.wantFiles {
				t.Fatalf("completionMessage() = (%q, %v), want (%q, %v)", gotText, gotFiles, tt.wantText, tt.wantFiles)
			}
		})
	}
}

func TestCompletionBeforeSendReturnsIsDeliveredExactlyOnce(t *testing.T) {
	manager, server := newManagerForTest(t, "token")
	defer manager.Stop()
	root := t.TempDir()
	var accepted Turn
	manager.SetRuntime(Runtime{
		Prepare: func(context.Context, string, string) (Turn, error) {
			return Turn{WorkspaceID: "workspace", WorkspacePath: root, SessionID: "session"}, nil
		},
		Run: func(_ context.Context, run Turn, _ string) error {
			accepted = run
			manager.Done(Completion{RunID: run.ID, SessionID: run.SessionID, Text: "early completion"})
			manager.Done(Completion{RunID: run.ID, SessionID: run.SessionID, Text: "duplicate completion"})
			return errors.New("response arrived after completion")
		},
	})
	manager.mu.Lock()
	manager.state.PairedChatIDs = []string{"chat"}
	manager.mu.Unlock()
	manager.dispatch(context.Background(), mustClient(t, manager, "token"), Update{ChatID: "chat", Text: "run"})
	if accepted.ID == "" {
		t.Fatal("missing run identity")
	}
	if !waitFor(t, time.Second, func() bool {
		for _, text := range server.deliveredMessages(t) {
			if text == "early completion" {
				return true
			}
		}
		return false
	}) {
		t.Fatal("completion was lost")
	}
	for _, text := range server.deliveredMessages(t) {
		if strings.Contains(text, "duplicate completion") || strings.Contains(text, "response arrived") {
			t.Fatalf("duplicate/error reply: %s", text)
		}
	}
}

func TestOldCompletionCannotFinishNewTurnInSameSession(t *testing.T) {
	manager, server := newManagerForTest(t, "token")
	defer manager.Stop()
	manager.mu.Lock()
	manager.state.PairedChatIDs = []string{"chat"}
	active := testActiveTurn("session")
	active.ID = "new-run"
	manager.active["chat"] = active
	manager.mu.Unlock()
	manager.Done(Completion{RunID: "old-run", SessionID: "session", Text: "wrong result"})
	manager.mu.Lock()
	unchanged := manager.active["chat"] == active && !active.completed
	manager.mu.Unlock()
	if !unchanged || len(server.deliveredMessages(t)) != 0 {
		t.Fatal("stale event completed another run")
	}
	manager.Done(Completion{RunID: "new-run", SessionID: "session", Error: "provider unavailable"})
	if !waitFor(t, time.Second, func() bool {
		for _, text := range server.deliveredMessages(t) {
			if strings.Contains(text, "provider unavailable") {
				return true
			}
		}
		return false
	}) {
		t.Fatal("failure was not reported")
	}
	for _, text := range server.deliveredMessages(t) {
		if text == "Done." {
			t.Fatal("failure reported as success")
		}
	}
}

func TestChangingTokenDoesNotRetainRemotePairings(t *testing.T) {
	manager, _ := newManagerForTest(t, "first")
	defer manager.Stop()
	manager.mu.Lock()
	manager.state.PairedChatIDs = []string{"old-chat"}
	manager.mu.Unlock()
	if _, err := manager.SetToken(context.Background(), "second"); err != nil {
		t.Fatal(err)
	}
	if manager.paired("old-chat") || manager.Status().Running {
		t.Fatal("new bot inherited access or started implicitly")
	}
}

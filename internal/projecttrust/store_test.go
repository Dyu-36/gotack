package projecttrust

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectRequiresDecisionForProtectedResources(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := New(filepath.Join(t.TempDir(), "trust.json"))
	status, err := store.Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Required || status.Trusted || status.Decided {
		t.Fatalf("unexpected initial status: %+v", status)
	}
	if err := store.Set(root, true); err != nil {
		t.Fatal(err)
	}
	status, err = store.Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if status.Required || !status.Trusted || !status.Decided {
		t.Fatalf("unexpected trusted status: %+v", status)
	}
}

func TestInspectInheritsClosestParentDecision(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(filepath.Join(child, ".pi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, ".pi", "SYSTEM.md"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := New(filepath.Join(t.TempDir(), "trust.json"))
	if err := store.Set(parent, false); err != nil {
		t.Fatal(err)
	}
	status, err := store.Inspect(child)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Decided || status.Required || status.Trusted || status.InheritedFrom == "" {
		t.Fatalf("unexpected inherited denial: %+v", status)
	}
}

func TestClearRemovesDecision(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := New(filepath.Join(t.TempDir(), "trust.json"))
	if err := store.Set(root, true); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(root); err != nil {
		t.Fatal(err)
	}
	status, err := store.Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if status.Decided || !status.Required || status.Trusted {
		t.Fatalf("unexpected status after clear: %+v", status)
	}
}

func TestClearPreservesEmptyDecisionState(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
	}{
		{name: "absent"},
		{name: "explicit null", data: `{"decisions":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".agents", "skills"), 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "trust.json")
			if tc.data != "" {
				if err := os.WriteFile(path, []byte(tc.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			if err := New(path).Clear(root); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var state map[string]json.RawMessage
			if err := json.Unmarshal(data, &state); err != nil {
				t.Fatalf("saved state is invalid JSON: %v", err)
			}
			if string(state["decisions"]) != "{}" {
				t.Fatalf("decisions = %s, want {}", state["decisions"])
			}
		})
	}
}

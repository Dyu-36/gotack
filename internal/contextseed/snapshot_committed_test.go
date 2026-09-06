package contextseed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotRejectsCorruptCommittedGeneration(t *testing.T) {
	for _, adoption := range []bool{false, true} {
		for _, mutation := range []string{"modified", "missing", "extra"} {
			name := mutation + "/reuse"
			if adoption {
				name = mutation + "/adopt"
			}
			t.Run(name, func(t *testing.T) {
				s := New(t.TempDir(), nil)
				seedContextWithFile(t, s, "TACK_CORE.md", "prior policy")
				prior, err := s.BuildPromptSnapshot()
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(s.ContextDir(), "TACK_CORE.md"), []byte("next policy"), 0600); err != nil {
					t.Fatal(err)
				}
				next, err := s.BuildPromptSnapshot()
				if err != nil {
					t.Fatal(err)
				}
				corrupt := func(dir string) {
					t.Helper()
					var err error
					switch mutation {
					case "modified":
						err = os.WriteFile(filepath.Join(dir, "TACK_CORE.md"), []byte("bad policy!"), 0600)
					case "missing":
						err = os.Remove(filepath.Join(dir, "TACK_CORE.md"))
					case "extra":
						err = os.WriteFile(filepath.Join(dir, "extra.md"), []byte("unexpected policy"), 0600)
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				if adoption {
					// Re-create a competing publisher's destination after our existence
					// check, before rename. Keep the staging manifest itself valid.
					hidden := next + ".fixture"
					if err := os.Rename(next, hidden); err != nil {
						t.Fatal(err)
					}
					withStagedHook(t, func(string) {
						if err := os.Rename(hidden, next); err != nil {
							t.Fatal(err)
						}
						corrupt(next)
					})
				} else {
					corrupt(next)
				}
				if _, err := s.BuildPromptSnapshot(); err == nil {
					t.Fatal("accepted committed generation that does not match manifest")
				} else if !strings.Contains(err.Error(), "validate") {
					t.Fatalf("wrong rejection: %v", err)
				}
				if got := string(mustRead(t, filepath.Join(prior, "TACK_CORE.md"))); got != "prior policy" {
					t.Fatal("previous revision changed on validation failure")
				}
				if _, err := os.Stat(next); err != nil {
					t.Fatal("corrupt generation was removed")
				}
			})
		}
	}
}

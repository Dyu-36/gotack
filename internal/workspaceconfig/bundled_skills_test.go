package workspaceconfig

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/workspace"
)

func TestBundledSkillsAreInstalledBeforeUserOverrides(t *testing.T) {
	root := t.TempDir()
	user := filepath.Join(root, "skills")
	projects := ProjectSkillsDirs(root)
	custom := filepath.Join(root, "custom-skills")
	old := filepath.Join(root, "bundled-skills", "old")
	current := filepath.Join(root, "bundled-skills", "current")
	cfg := engineapi.WorkspaceConfig{Options: &engineapi.WorkspaceOptions{SkillsPaths: []string{user, old, projects[0], projects[1], custom}}}
	writes := map[string]json.RawMessage{}
	var removals []string
	client := configClient(t, &cfg, &writes, &removals)
	desc := workspace.Descriptor{WorkspaceID: "ws", Path: root}
	if err := RegisterSkillsPathsWithTrust(context.Background(), client, "ws", desc, user, true, current); err != nil {
		t.Fatal(err)
	}
	want := []string{current, custom, user, projects[0], projects[1]}
	if !reflect.DeepEqual(cfg.SkillsPaths(), want) {
		t.Fatalf("paths = %v, want %v", cfg.SkillsPaths(), want)
	}
	clear(writes)
	if err := RegisterSkillsPathsWithTrust(context.Background(), client, "ws", desc, user, true, current); err != nil {
		t.Fatal(err)
	}
	if len(writes) != 0 {
		t.Fatalf("idempotent registration wrote %v", writes)
	}
}

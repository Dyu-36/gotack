package workspaceconfig

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/workspace"
)

type configTransport func(*http.Request) (*http.Response, error)

func (f configTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func configClient(t *testing.T, cfg *engineapi.WorkspaceConfig, writes *map[string]json.RawMessage, removals *[]string) *engineapi.Client {
	t.Helper()
	return engineapi.NewClient(&http.Client{Transport: configTransport(func(r *http.Request) (*http.Response, error) {
		body := "{}"
		if r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws/config" {
			data, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			body = string(data)
		} else if r.Method == http.MethodPost {
			var request struct {
				Scope int
				Key   string
				Value json.RawMessage
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.Scope != engineapi.ConfigScopeWorkspace {
				t.Fatalf("scope = %d", request.Scope)
			}
			switch r.URL.Path {
			case "/v1/workspaces/ws/config/remove":
				*removals = append(*removals, request.Key)
			case "/v1/workspaces/ws/config/set":
				(*writes)[request.Key] = request.Value
				if request.Key == "options.skills_paths" {
					if cfg.Options == nil {
						cfg.Options = &engineapi.WorkspaceOptions{}
					}
					if err := json.Unmarshal(request.Value, &cfg.Options.SkillsPaths); err != nil {
						t.Fatal(err)
					}
				}
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		} else {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})})
}

func TestManagedContextPathsAreNarrowlyScoped(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		path string
		want bool
	}{
		{filepath.Join(root, "context-prompt", "generation"), true},
		{filepath.Join(root, "context-prompt"), true},
		{filepath.Join(root, "context-prompt-custom", "instructions.md"), false},
		{filepath.Join(root, "instructions.md"), false},
		{filepath.Join(root+"-backup", "context-prompt"), false},
		{"context-prompt", false},
	} {
		if got := isManagedPath(tc.path, root); got != tc.want {
			t.Errorf("isManagedPath(%q) = %v", tc.path, got)
		}
	}
}

func TestLegacyEnvironmentPreservesUserPaths(t *testing.T) {
	root := t.TempDir()
	sep := string(filepath.ListSeparator)
	userPath := filepath.Join(root, "my-officecli-tools")
	env := map[string]string{"Path": strings.Join([]string{filepath.Join(root, "bin"), userPath, filepath.Join(root, "binary")}, sep), "TOKEN": "keep"}
	original := env["Path"]
	got := legacyEnvWithoutOffice(env, root)
	if got["Path"] != strings.Join([]string{userPath, filepath.Join(root, "binary")}, sep) || got["TOKEN"] != "keep" {
		t.Fatalf("cleaned env = %v", got)
	}
	if env["Path"] != original {
		t.Fatal("input mutated")
	}
	if legacyEnvWithoutOffice(got, root) != nil {
		t.Fatal("cleanup not idempotent")
	}
	if legacyEnvWithoutOffice(env, "") != nil {
		t.Fatal("unknown managed root must preserve env")
	}
}

func TestSkillsRegistrationReplacesDuplicatesWithoutLosingAdditions(t *testing.T) {
	root := t.TempDir()
	user := filepath.Join(root, "skills")
	cfg := engineapi.WorkspaceConfig{Options: &engineapi.WorkspaceOptions{SkillsPaths: []string{user, user}}}
	writes := map[string]json.RawMessage{}
	var removals []string
	client := configClient(t, &cfg, &writes, &removals)
	desc := workspace.Descriptor{WorkspaceID: "ws", Path: root}
	if err := RegisterSkillsPaths(context.Background(), client, "ws", desc, user); err != nil {
		t.Fatal(err)
	}
	want := append([]string{user}, ProjectSkillsDirs(root)...)
	if !reflect.DeepEqual(cfg.SkillsPaths(), want) {
		t.Fatalf("paths = %v, want %v", cfg.SkillsPaths(), want)
	}
	clear(writes)
	if err := RegisterSkillsPaths(context.Background(), client, "ws", desc, user); err != nil {
		t.Fatal(err)
	}
	if len(writes) != 0 {
		t.Fatalf("idempotent registration wrote %v", writes)
	}
}

func TestMigrationPreservesUserHooksAndContext(t *testing.T) {
	root := t.TempDir()
	user := filepath.Join(root, "instructions.md")
	hook := engineapi.HookEntry{Name: "user-hook", Command: "keep"}
	cfg := engineapi.WorkspaceConfig{
		Hooks:   map[string][]engineapi.HookEntry{LegacyGuardHookEvent: {{Name: LegacyGuardHookName}, hook}},
		Options: &engineapi.WorkspaceOptions{GlobalContextPaths: []string{filepath.Join(root, "context-prompt", "generation"), user}},
		Env:     map[string]string{"PATH": filepath.Join(root, "bin")},
	}
	writes := map[string]json.RawMessage{}
	var removals []string
	client := configClient(t, &cfg, &writes, &removals)
	if err := removeLegacyTools(context.Background(), client, "ws", root); err != nil {
		t.Fatal(err)
	}
	want := []string{"mcp_servers.gotack-memory", "mcp_servers.gotack-skills", "mcp_servers.gotack-recall", "mcp_servers.gotack-office"}
	if !reflect.DeepEqual(removals, want) {
		t.Fatalf("removals = %v", removals)
	}
	var hooks []engineapi.HookEntry
	if err := json.Unmarshal(writes[LegacyGuardHookKey], &hooks); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(hooks, []engineapi.HookEntry{hook}) {
		t.Fatalf("hooks = %v", hooks)
	}
	var paths []string
	if err := json.Unmarshal(writes["options.global_context_paths"], &paths); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(paths, []string{user}) {
		t.Fatalf("paths = %v", paths)
	}
}

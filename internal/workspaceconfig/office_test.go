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

type officeRuntimeStub struct {
	env        map[string]string
	skillsPath string
}

func (s officeRuntimeStub) EngineEnv() map[string]string { return s.env }
func (s officeRuntimeStub) SkillsPath() string           { return s.skillsPath }

type officeConfigAPI struct {
	t             *testing.T
	existing      []string
	existingEnv   map[string]string
	writtenSkills []string
	writtenEnv    map[string]string
	removedLegacy bool
}

func (f *officeConfigAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/config/remove"):
		var payload struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			f.t.Fatalf("decode config remove request: %v", err)
		}
		f.removedLegacy = payload.Key == "mcp_servers."+LegacyOfficeMCPName
		return testHTTPResponse(http.StatusOK, `{}`), nil
	case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/config"):
		config := map[string]any{}
		if len(f.existing) > 0 {
			config["options"] = map[string]any{"skills_paths": f.existing}
		}
		if len(f.existingEnv) > 0 {
			config["env"] = f.existingEnv
		}
		body, err := json.Marshal(config)
		if err != nil {
			f.t.Fatalf("encode fake config: %v", err)
		}
		return testHTTPResponse(http.StatusOK, string(body)), nil
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/config/set-batch"):
		var payload struct {
			Fields map[string]json.RawMessage `json:"fields"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			f.t.Fatalf("decode config batch request: %v", err)
		}
		if raw := payload.Fields["options.skills_paths"]; raw != nil {
			if err := json.Unmarshal(raw, &f.writtenSkills); err != nil {
				f.t.Fatalf("decode skills_paths: %v", err)
			}
		}
		if raw := payload.Fields["env"]; raw != nil {
			if err := json.Unmarshal(raw, &f.writtenEnv); err != nil {
				f.t.Fatalf("decode env: %v", err)
			}
		}
		return testHTTPResponse(http.StatusOK, `{}`), nil
	default:
		f.t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return testHTTPResponse(http.StatusNotFound, `{}`), nil
	}
}

func testHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestMergeSkillsPaths(t *testing.T) {
	tests := []struct {
		name      string
		existing  []string
		additions []string
		want      []string
	}{
		{"append bundled", []string{"~/user/skills-a", "D:/user/skills-b"}, []string{"C:/gotack/skills"}, []string{"~/user/skills-a", "D:/user/skills-b", "C:/gotack/skills"}},
		{"dedupe bundled", []string{"~/user/skills-a", "C:/gotack/skills"}, []string{"C:/gotack/skills"}, []string{"~/user/skills-a", "C:/gotack/skills"}},
		{"empty config", nil, []string{"C:/gotack/skills"}, []string{"C:/gotack/skills"}},
		{"dedupe additions", nil, []string{"C:/gotack/skills", "C:/gotack/skills", "P:/ws/.agents/skills"}, []string{"C:/gotack/skills", "P:/ws/.agents/skills"}},
		{"skip empty", []string{"~/user/skills-a"}, []string{"", "C:/gotack/skills"}, []string{"~/user/skills-a", "C:/gotack/skills"}},
		{"dedupe existing", []string{"C:/gotack/skills", "C:/gotack/skills", "D:/user/skills"}, nil, []string{"C:/gotack/skills", "D:/user/skills"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := MergeSkillsPaths(tc.existing, tc.additions...); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("MergeSkillsPaths(%#v, %#v) = %#v, want %#v", tc.existing, tc.additions, got, tc.want)
			}
		})
	}
}

func TestMergeConfigEnvPreservesUserKeys(t *testing.T) {
	existing := map[string]string{"CUSTOM": "keep", "PATH": "user-path"}
	got := MergeConfigEnv(existing, map[string]string{"PATH": "gotack-path"})
	want := map[string]string{"CUSTOM": "keep", "PATH": "gotack-path"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeConfigEnv() = %#v, want %#v", got, want)
	}
	if existing["PATH"] != "user-path" {
		t.Fatalf("MergeConfigEnv mutated input: %#v", existing)
	}
}

func TestRegisterOfficePreservesConfigAndAddsSkillPaths(t *testing.T) {
	fake := &officeConfigAPI{
		t:           t,
		existing:    []string{"~/user/skills-a"},
		existingEnv: map[string]string{"CUSTOM": "keep", "PATH": "user-path"},
	}
	api := engineapi.NewClient(&http.Client{Transport: fake})
	runtime := officeRuntimeStub{
		env:        map[string]string{"PATH": "gotack-path"},
		skillsPath: "C:/gotack/skills",
	}
	desc := workspace.Descriptor{Path: filepath.FromSlash("P:/ws"), WorkspaceID: "ws-1"}

	if err := RegisterOffice(context.Background(), api, "ws-1", desc, runtime, "U:/gotack/skills"); err != nil {
		t.Fatalf("RegisterOffice: %v", err)
	}
	if !fake.removedLegacy {
		t.Fatal("RegisterOffice did not remove legacy Office MCP config")
	}
	if fake.writtenEnv["CUSTOM"] != "keep" || fake.writtenEnv["PATH"] != "gotack-path" {
		t.Fatalf("written env = %#v", fake.writtenEnv)
	}
	wantSkills := []string{"~/user/skills-a", "C:/gotack/skills", "U:/gotack/skills", ProjectSkillsDir(desc.Path)}
	if !reflect.DeepEqual(fake.writtenSkills, wantSkills) {
		t.Fatalf("written skills = %#v, want %#v", fake.writtenSkills, wantSkills)
	}
}

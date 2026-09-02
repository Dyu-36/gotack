package main

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/officecli"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/workspace"
)

func TestMergeSkillsPaths(t *testing.T) {
	tests := []struct {
		name      string
		existing  []string
		additions []string
		want      []string
	}{
		{
			name:      "user paths keep their order and bundled is appended",
			existing:  []string{"~/user/skills-a", "D:/user/skills-b"},
			additions: []string{"C:/gotack/skills"},
			want:      []string{"~/user/skills-a", "D:/user/skills-b", "C:/gotack/skills"},
		},
		{
			name:      "bundled already present is not duplicated",
			existing:  []string{"~/user/skills-a", "C:/gotack/skills"},
			additions: []string{"C:/gotack/skills"},
			want:      []string{"~/user/skills-a", "C:/gotack/skills"},
		},
		{
			name:      "empty config gets only the bundled path",
			existing:  nil,
			additions: []string{"C:/gotack/skills"},
			want:      []string{"C:/gotack/skills"},
		},
		{
			name:      "multiple additions append once each in order",
			existing:  []string{"~/user/skills-a"},
			additions: []string{"C:/gotack/skills", "U:/gotack/skills", "P:/ws/.agents/skills"},
			want:      []string{"~/user/skills-a", "C:/gotack/skills", "U:/gotack/skills", "P:/ws/.agents/skills"},
		},
		{
			name:      "duplicate additions are appended once",
			existing:  nil,
			additions: []string{"C:/gotack/skills", "C:/gotack/skills", "P:/ws/.agents/skills"},
			want:      []string{"C:/gotack/skills", "P:/ws/.agents/skills"},
		},
		{
			name:      "empty additions are skipped",
			existing:  []string{"~/user/skills-a"},
			additions: []string{"", "C:/gotack/skills"},
			want:      []string{"~/user/skills-a", "C:/gotack/skills"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := mergeSkillsPaths(test.existing, test.additions...)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("mergeSkillsPaths(%#v, %#v) = %#v, want %#v", test.existing, test.additions, got, test.want)
			}
		})
	}
}

// skillsPathsAPI is a fake Crush transport that remembers the skills_paths
// value served by GET config and records every value written through config
// set, so the registration flow can be asserted end to end.
type skillsPathsAPI struct {
	t             *testing.T
	existing      []string
	existingEnv   map[string]string
	writtenKeys   []string
	writtenSkills []string
	wroteSkills   bool
	writtenEnv    map[string]string
}

func (f *skillsPathsAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces":
		return jsonHTTPResponse(http.StatusOK, `[]`), nil
	case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces":
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return jsonHTTPResponse(http.StatusBadRequest, `{"message":"bad request"}`), nil
		}
		out, _ := json.Marshal(map[string]string{"id": "ws-1", "path": body.Path})
		return jsonHTTPResponse(http.StatusOK, string(out)), nil
	case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/config"):
		config := map[string]any{}
		if len(f.existing) > 0 {
			config["options"] = map[string]any{"skills_paths": f.existing}
		}
		if len(f.existingEnv) > 0 {
			config["env"] = f.existingEnv
		}
		encoded, err := json.Marshal(config)
		if err != nil {
			f.t.Fatalf("encode fake config: %v", err)
		}
		return jsonHTTPResponse(http.StatusOK, string(encoded)), nil
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/config/set"):
		var payload struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			f.t.Errorf("decode config set request: %v", err)
			return jsonHTTPResponse(http.StatusBadRequest, `{"message":"bad request"}`), nil
		}
		f.writtenKeys = append(f.writtenKeys, payload.Key)
		if payload.Key == "options.skills_paths" {
			var paths []string
			if err := json.Unmarshal(payload.Value, &paths); err != nil {
				f.t.Errorf("decode skills_paths write: %v", err)
			}
			f.writtenSkills = paths
			f.wroteSkills = true
		}
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/config/set-batch"):
		var payload struct {
			Fields map[string]json.RawMessage `json:"fields"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			f.t.Errorf("decode config batch request: %v", err)
			return jsonHTTPResponse(http.StatusBadRequest, `{"message":"bad request"}`), nil
		}
		for key, raw := range payload.Fields {
			f.writtenKeys = append(f.writtenKeys, key)
			switch key {
			case "options.skills_paths":
				if err := json.Unmarshal(raw, &f.writtenSkills); err != nil {
					f.t.Errorf("decode skills_paths batch write: %v", err)
				}
				f.wroteSkills = true
			case "env":
				if err := json.Unmarshal(raw, &f.writtenEnv); err != nil {
					f.t.Errorf("decode env batch write: %v", err)
				}
			}
		}
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/config/remove"):
		// registerOfficeTools removes the office MCP entry when the binary
		// is missing; the skills_paths assertion does not depend on it.
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	default:
		f.t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
	}
}

func TestMergeConfigEnvPreservesUserKeys(t *testing.T) {
	existing := map[string]string{"CUSTOM": "keep", "PATH": "user-path"}
	got := mergeConfigEnv(existing, map[string]string{"PATH": "gotack-path"})
	want := map[string]string{"CUSTOM": "keep", "PATH": "gotack-path"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeConfigEnv() = %#v, want %#v", got, want)
	}
	if existing["PATH"] != "user-path" {
		t.Fatalf("mergeConfigEnv mutated input: %#v", existing)
	}
}

func TestRegisterOfficeToolsPreservesExistingSkillsPaths(t *testing.T) {
	tests := []struct {
		name     string
		existing func(bundled string) []string
		want     func(bundled string) []string
	}{
		{
			name:     "two preconfigured paths all remain plus bundled",
			existing: func(string) []string { return []string{"~/user/skills-a", "D:/user/skills-b"} },
			want:     func(bundled string) []string { return []string{"~/user/skills-a", "D:/user/skills-b", bundled} },
		},
		{
			name:     "bundled path already present is not duplicated",
			existing: func(bundled string) []string { return []string{"~/user/skills-a", bundled} },
			want:     func(bundled string) []string { return []string{"~/user/skills-a", bundled} },
		},
		{
			name:     "empty config gets just the bundled path",
			existing: func(string) []string { return nil },
			want:     func(bundled string) []string { return []string{bundled} },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The seeded and user skills directories intentionally coincide;
			// production must keep only one entry.
			appData := redirectAppData(t)
			dataDir := filepath.Join(appData, "gotack")
			bundled := filepath.Join(dataDir, "skills")
			fake := &skillsPathsAPI{
				t:           t,
				existing:    test.existing(bundled),
				existingEnv: map[string]string{"CUSTOM": "keep"},
			}

			api := crushapi.NewClient(&http.Client{Transport: fake})
			app := NewApp()
			app.ctx = context.Background()
			app.officeSeeder = &officeSeeder{seeder: officecli.New(dataDir, nil)}
			app.swapConn(func(c *conn) *conn {
				c.api = api
				c.ws = workspace.NewService(api)
				c.sess = session.NewService(api, c.ws)
				return c
			})
			// Drive the link through the same transitions connect() uses so the
			// services above count as a live engine connection.
			scope, started := app.link.BeginConnect(context.Background())
			if !started || !app.link.CommitAttach(scope, crushapi.Endpoint{}, "test") {
				t.Fatal("link rejected the test connect scope")
			}
			app.link.MarkRunning()

			app.registerOfficeTools("ws-1")

			if !fake.wroteSkills {
				t.Fatalf("registerOfficeTools never wrote options.skills_paths; keys written: %v", fake.writtenKeys)
			}
			want := test.want(bundled)
			if !reflect.DeepEqual(fake.writtenSkills, want) {
				t.Fatalf("written options.skills_paths = %#v, want %#v", fake.writtenSkills, want)
			}
			if fake.writtenEnv["CUSTOM"] != "keep" || fake.writtenEnv["PATH"] == "" {
				t.Fatalf("written env did not preserve user values and add PATH: %#v", fake.writtenEnv)
			}
		})
	}
}

func TestRegisterOfficeToolsAppendsUserAndProjectSkillsDirs(t *testing.T) {
	appData := redirectAppData(t)
	dataDir := filepath.Join(appData, "gotack")
	bundled := filepath.Join(dataDir, "skills")
	fake := &skillsPathsAPI{t: t, existing: []string{"~/user/skills-a"}}

	api := crushapi.NewClient(&http.Client{Transport: fake})
	app := NewApp()
	app.ctx = context.Background()
	app.officeSeeder = &officeSeeder{seeder: officecli.New(dataDir, nil)}
	app.swapConn(func(c *conn) *conn {
		c.api = api
		c.ws = workspace.NewService(api)
		c.sess = session.NewService(api, c.ws)
		return c
	})
	scope, started := app.link.BeginConnect(context.Background())
	if !started || !app.link.CommitAttach(scope, crushapi.Endpoint{}, "test") {
		t.Fatal("link rejected the test connect scope")
	}
	app.link.MarkRunning()
	wsDir := t.TempDir()
	if _, err := app.getConn().ws.Open(context.Background(), wsDir); err != nil {
		t.Fatalf("open workspace: %v", err)
	}

	app.registerOfficeTools("ws-1")

	if !fake.wroteSkills {
		t.Fatalf("registerOfficeTools never wrote options.skills_paths; keys written: %v", fake.writtenKeys)
	}
	want := []string{"~/user/skills-a", bundled, filepath.Join(wsDir, ".agents", "skills")}
	if !reflect.DeepEqual(fake.writtenSkills, want) {
		t.Fatalf("written options.skills_paths = %#v, want %#v", fake.writtenSkills, want)
	}
}

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
	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/officecli"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/workspace"
)

func TestMergeSkillsPaths(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		bundled  string
		want     []string
	}{
		{
			name:     "user paths keep their order and bundled is appended",
			existing: []string{"~/user/skills-a", "D:/user/skills-b"},
			bundled:  "C:/gotack/skills",
			want:     []string{"~/user/skills-a", "D:/user/skills-b", "C:/gotack/skills"},
		},
		{
			name:     "bundled already present is not duplicated",
			existing: []string{"~/user/skills-a", "C:/gotack/skills"},
			bundled:  "C:/gotack/skills",
			want:     []string{"~/user/skills-a", "C:/gotack/skills"},
		},
		{
			name:     "empty config gets only the bundled path",
			existing: nil,
			bundled:  "C:/gotack/skills",
			want:     []string{"C:/gotack/skills"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := mergeSkillsPaths(test.existing, test.bundled)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("mergeSkillsPaths(%#v, %q) = %#v, want %#v", test.existing, test.bundled, got, test.want)
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
	writtenKeys   []string
	writtenSkills []string
	wroteSkills   bool
}

func (f *skillsPathsAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/config"):
		body := "{}"
		if len(f.existing) > 0 {
			options, err := json.Marshal(map[string]any{"options": map[string]any{"skills_paths": f.existing}})
			if err != nil {
				f.t.Fatalf("encode fake config: %v", err)
			}
			body = string(options)
		}
		return jsonHTTPResponse(http.StatusOK, body), nil
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
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/config/remove"):
		// registerOfficeTools removes the office MCP entry when the binary
		// is missing; the skills_paths assertion does not depend on it.
		return jsonHTTPResponse(http.StatusOK, `{}`), nil
	default:
		f.t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
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
			dataDir := t.TempDir()
			bundled := filepath.Join(dataDir, "skills")
			fake := &skillsPathsAPI{t: t, existing: test.existing(bundled)}

			api := crushapi.NewClient(&http.Client{Transport: fake})
			app := NewApp()
			app.ctx = context.Background()
			app.officeSeeder = &officeSeeder{seeder: officecli.New(dataDir, nil)}
			app.swapConn(func(c *conn) *conn {
				c.api = api
				c.ws = workspace.NewService(api)
				c.sess = session.NewService(api, c.ws)
				c.status = engine.StatusRunning
				return c
			})

			app.registerOfficeTools("ws-1")

			if !fake.wroteSkills {
				t.Fatalf("registerOfficeTools never wrote options.skills_paths; keys written: %v", fake.writtenKeys)
			}
			want := test.want(bundled)
			if !reflect.DeepEqual(fake.writtenSkills, want) {
				t.Fatalf("written options.skills_paths = %#v, want %#v", fake.writtenSkills, want)
			}
		})
	}
}

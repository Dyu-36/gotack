package appconfig

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAddRecentWorkspace(t *testing.T) {
	cases := []struct {
		name string
		init []string
		add  string
		want []string
	}{
		{
			name: "empty list, add new",
			init: nil,
			add:  "/home/u/proj",
			want: []string{"/home/u/proj"},
		},
		{
			name: "add to front, dedupe prior",
			init: []string{"/a", "/b"},
			add:  "/c",
			want: []string{"/c", "/a", "/b"},
		},
		{
			name: "promote existing entry to front",
			init: []string{"/a", "/b", "/c"},
			add:  "/b",
			want: []string{"/b", "/a", "/c"},
		},
		{
			name: "normalize trailing slash",
			init: []string{"/a", "/b"},
			add:  "/a/",
			want: []string{"/a", "/b"},
		},
		{
			name: "cap at 10",
			init: []string{"/1", "/2", "/3", "/4", "/5", "/6", "/7", "/8", "/9"},
			add:  "/new",
			want: []string{"/new", "/1", "/2", "/3", "/4", "/5", "/6", "/7", "/8", "/9"},
		},
		{
			name: "drop beyond cap when promoting",
			init: []string{"/1", "/2", "/3", "/4", "/5", "/6", "/7", "/8", "/9", "/10"},
			add:  "/3",
			want: []string{"/3", "/1", "/2", "/4", "/5", "/6", "/7", "/8", "/9", "/10"},
		},
		{
			name: "empty path ignored",
			init: []string{"/a"},
			add:  "",
			want: []string{"/a"},
		},
		{
			name: "nil config tolerated (no panic)",
			init: nil,
			add:  "/x",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg *Config
			if !strings.HasPrefix(tc.name, "nil config tolerated") {
				cfg = &Config{RecentWorkspaces: append([]string(nil), tc.init...)}
			}
			AddRecentWorkspace(cfg, tc.add)
			if cfg == nil {
				if tc.want != nil {
					t.Fatalf("nil cfg but expected %v", tc.want)
				}
				return
			}
			if !reflect.DeepEqual(cfg.RecentWorkspaces, tc.want) {
				t.Fatalf("got %v want %v", cfg.RecentWorkspaces, tc.want)
			}
			if len(cfg.RecentWorkspaces) > maxRecent {
				t.Fatalf("exceeded cap: %d", len(cfg.RecentWorkspaces))
			}
		})
	}
}

func TestDefaults(t *testing.T) {
	d := Defaults()
	if d.Theme != "system" {
		t.Errorf("Theme=%q want system", d.Theme)
	}
	if d.EngineBinary != "" {
		t.Errorf("EngineBinary=%q want empty", d.EngineBinary)
	}
	if d.RecentWorkspaces != nil {
		t.Errorf("RecentWorkspaces=%v want nil", d.RecentWorkspaces)
	}
	if d.Provider != "" || d.Model != "" || d.Thinking != "" {
		t.Errorf("agent settings must default empty so Crush catalog defaults apply, got provider=%q model=%q thinking=%q", d.Provider, d.Model, d.Thinking)
	}
	if d.Zalo.Enabled || d.Zalo.Token != "" {
		t.Errorf("Zalo must default disabled with no token, got %+v", d.Zalo)
	}
}

func TestAddRecentWorkspaceIgnoresCleanVariants(t *testing.T) {
	cfg := &Config{RecentWorkspaces: []string{filepath.Clean("/tmp/proj")}}
	AddRecentWorkspace(cfg, "/tmp//proj/.")
	if len(cfg.RecentWorkspaces) != 1 {
		t.Fatalf("expected dedupe, got %v", cfg.RecentWorkspaces)
	}
	if filepath.Clean(cfg.RecentWorkspaces[0]) != filepath.Clean("/tmp/proj") {
		t.Fatalf("front entry not normalized: %q", cfg.RecentWorkspaces[0])
	}
}

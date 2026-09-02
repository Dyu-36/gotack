package appconfig

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
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
	if d.AutoApprove {
		t.Errorf("AutoApprove=%v want false", d.AutoApprove)
	}
	if d.Zalo.Enabled || d.Zalo.Token != "" {
		t.Errorf("Zalo must default disabled with no token, got %+v", d.Zalo)
	}
}

func TestAutoApproveRequiresExplicitTrue(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		want bool
	}{
		{name: "missing", data: `{}`, want: false},
		{name: "explicit false", data: `{"auto_approve":false}`, want: false},
		{name: "explicit true", data: `{"auto_approve":true}`, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Defaults()
			if err := json.Unmarshal([]byte(tc.data), cfg); err != nil {
				t.Fatalf("unmarshal config: %v", err)
			}
			if cfg.AutoApprove != tc.want {
				t.Fatalf("AutoApprove = %v, want %v", cfg.AutoApprove, tc.want)
			}
		})
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

// TestZaloLegacyFieldsCarryDeprecationNotice guards the A8 contract: the
// legacy Zalo keys stay doc-deprecated until the Gotack v1.0 removal target,
// because zalo.Manager.ImportLegacy still consumes them at startup.
func TestZaloLegacyFieldsCarryDeprecationNotice(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "config.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse config.go: %v", err)
	}
	notices := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok || spec.Name.Name != "ZaloSettings" {
			return true
		}
		layout, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range layout.Fields.List {
			if field.Doc == nil || len(field.Names) == 0 {
				continue
			}
			if strings.Contains(field.Doc.Text(), "Deprecated:") {
				notices[field.Names[0].Name] = true
			}
		}
		return true
	})
	for _, name := range []string{"Token", "AllowedChats"} {
		if !notices[name] {
			t.Errorf("ZaloSettings.%s lost its // Deprecated: notice; ImportLegacy still migrates it", name)
		}
	}
}

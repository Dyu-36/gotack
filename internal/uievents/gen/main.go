//go:build ignore

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	root, _ := filepath.Abs("internal/uievents")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, root, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse:", err)
		os.Exit(1)
	}
	pkg, ok := pkgs["uievents"]
	if !ok {
		fmt.Fprintln(os.Stderr, "no uievents package")
		os.Exit(1)
	}
	var events [][2]string
	for _, file := range pkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			gen, ok := n.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				return true
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, ident := range vs.Names {
					if len(vs.Values) == 0 {
						continue
					}
					bl, ok := vs.Values[0].(*ast.BasicLit)
					if !ok || bl.Kind != token.STRING {
						continue
					}
					s, err := strconv.Unquote(bl.Value)
					if err != nil {
						continue
					}
					events = append(events, [2]string{ident.Name, s})
				}
			}
			return true
		})
	}
	sort.Slice(events, func(i, j int) bool { return events[i][0] < events[j][0] })

	var sb strings.Builder
	sb.WriteString("// events.generated.ts -- DO NOT EDIT.\n")
	sb.WriteString("// Source: internal/uievents/names.go\n")
	sb.WriteString("// Regenerate with: go run ./internal/uievents/gen/main.go\n\n")
	sb.WriteString("export const events = {\n")
	for _, e := range events {
		fmt.Fprintf(&sb, "  %s: %q,\n", toCamel(e[0]), e[1])
	}
	sb.WriteString("} as const\n")
	sb.WriteString("export type EventName = (typeof events)[keyof typeof events]\n")

	out := "frontend/src/platform/events.generated.ts"
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Println("wrote", out)
}

func toCamel(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

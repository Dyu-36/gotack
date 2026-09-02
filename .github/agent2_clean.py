from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    text = text.strip("\n") + "\n"
    if not path.exists() or path.read_text(encoding="utf-8") != text:
        path.write_text(text, encoding="utf-8")


def drop_import(text: str, import_path: str, qualifier: str) -> str:
    if qualifier in text:
        return text
    text = re.sub(rf"(?m)^\s*{re.escape(import_path)}\s*\n", "", text)
    return re.sub(rf"(?m)^import\s+{re.escape(import_path)}\s*\n", "", text)


def split_assets() -> None:
    pattern = re.compile(
        r"(?m)^//go:embed[^\n]*frontend/dist[^\n]*\n"
        r"var\s+([A-Za-z_]\w*)\s+embed\.FS\s*\n?"
    )
    variable: str | None = None
    production = False
    for path in sorted(ROOT.glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        text = path.read_text(encoding="utf-8")
        match = pattern.search(text)
        if not match:
            continue
        variable = variable or match.group(1)
        if re.search(r"(?m)^//go:build\s+production\s*$", text):
            production = True
            continue
        updated = pattern.sub("", text, count=1)
        write(path, drop_import(updated, '"embed"', "embed."))
    for path in ROOT.glob("frontend_assets_*.go"):
        text = path.read_text(encoding="utf-8")
        if re.search(r"(?m)^//go:build\s+production\s*$", text) and "frontend/dist" in text:
            production = True
            match = re.search(r"(?m)^var\s+([A-Za-z_]\w*)\s+embed\.FS\s*$", text)
            if match:
                variable = variable or match.group(1)
    if variable is None:
        for path in ROOT.glob("*.go"):
            match = re.search(
                r"AssetServer:\s*&assetserver\.Options\{\s*Assets:\s*([A-Za-z_]\w*)",
                path.read_text(encoding="utf-8"),
            )
            if match:
                variable = match.group(1)
                break
    if variable is None:
        raise RuntimeError("frontend asset variable not found")
    if not production:
        write(
            ROOT / "frontend_assets_production.go",
            f'''//go:build production

package main

import "embed"

//go:embed all:frontend/dist
var {variable} embed.FS
''',
        )
    tooling = any(
        re.search(r"(?m)^//go:build\s+!production\s*$", path.read_text(encoding="utf-8"))
        and re.search(rf"(?m)^var\s+{re.escape(variable)}\b", path.read_text(encoding="utf-8"))
        for path in ROOT.glob("frontend_assets_*.go")
    )
    if not tooling:
        write(
            ROOT / "frontend_assets_tooling.go",
            f'''//go:build !production

package main

import "testing/fstest"

var {variable} = fstest.MapFS{{
    "index.html": &fstest.MapFile{{Data: []byte("<!doctype html><title>Gotack</title>")}},
}}
''',
        )


def hide_callbacks() -> None:
    renames = {
        "RunDone": "runDone",
        "AssistantIteration": "assistantIteration",
        "LearningToolExecuted": "learningToolExecuted",
    }
    for path in ROOT.glob("*.go"):
        text = path.read_text(encoding="utf-8")
        updated = text
        for old, new in renames.items():
            updated = re.sub(rf"\b{old}\b", new, updated)
        if updated != text:
            write(path, updated)


def pkg(directory: Path) -> str:
    for path in directory.glob("*.go"):
        match = re.search(r"(?m)^package\s+([A-Za-z_]\w*)\s*$", path.read_text(encoding="utf-8"))
        if match:
            return match.group(1)
    raise RuntimeError(f"package not found in {directory}")


def portable_basenames() -> None:
    candidates = [ROOT, ROOT / "internal" / "attachments"]
    keywords = ("attachment", "Attachment", "displayName", "DisplayName", "fileName", "FileName")
    for directory in candidates:
        if not directory.exists():
            continue
        helper = directory / "portable_path.go"
        changed = False
        for path in directory.glob("*.go"):
            if path.name.endswith("_test.go") or path == helper:
                continue
            text = path.read_text(encoding="utf-8")
            if "filepath.Base(" not in text or not any(word in text for word in keywords):
                continue
            updated = text.replace("filepath.Base(", "portableBase(")
            write(path, drop_import(updated, '"path/filepath"', "filepath."))
            changed = True
        used = changed or any(
            "portableBase(" in path.read_text(encoding="utf-8")
            for path in directory.glob("*.go")
            if path != helper
        )
        if not used:
            continue
        package = pkg(directory)
        write(
            helper,
            f'''package {package}

import (
    pathpkg "path"
    "strings"
)

func portableBase(value string) string {{
    return pathpkg.Base(strings.ReplaceAll(value, "\\\\", "/"))
}}
''',
        )
        write(
            directory / "portable_path_test.go",
            f'''package {package}

import "testing"

func TestPortableBase(t *testing.T) {{
    t.Parallel()
    for name, input := range map[string]string{{
        "windows": `C:\\tmp\\photo.png`,
        "unix":    "/tmp/photo.png",
        "plain":   "photo.png",
    }} {{
        name, input := name, input
        t.Run(name, func(t *testing.T) {{
            t.Parallel()
            if got := portableBase(input); got != "photo.png" {{
                t.Fatalf("portableBase(%q) = %q, want photo.png", input, got)
            }}
        }})
    }}
}}
''',
        )


def remove_go_function(text: str, signature: re.Match[str]) -> str:
    start = signature.start()
    brace = text.find("{", signature.end())
    if brace < 0:
        raise RuntimeError("function body not found")
    depth = 0
    state = "code"
    escaped = False
    i = brace
    while i < len(text):
        ch = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ""
        if state == "code":
            if ch == "/" and nxt == "/":
                state = "line"; i += 1
            elif ch == "/" and nxt == "*":
                state = "block"; i += 1
            elif ch == '"':
                state = "string"; escaped = False
            elif ch == "`":
                state = "raw"
            elif ch == "'":
                state = "rune"; escaped = False
            elif ch == "{":
                depth += 1
            elif ch == "}":
                depth -= 1
                if depth == 0:
                    end = i + 1
                    while end < len(text) and text[end] in " \t\n":
                        end += 1
                    return text[:start] + text[end:]
        elif state == "line" and ch == "\n":
            state = "code"
        elif state == "block" and ch == "*" and nxt == "/":
            state = "code"; i += 1
        elif state in ("string", "rune"):
            if escaped:
                escaped = False
            elif ch == "\\":
                escaped = True
            elif (state == "string" and ch == '"') or (state == "rune" and ch == "'"):
                state = "code"
        elif state == "raw" and ch == "`":
            state = "code"
        i += 1
    raise RuntimeError("unterminated function")


def remove_dead_browse() -> None:
    path = ROOT / "internal" / "recall" / "session.go"
    if not path.exists():
        return
    text = path.read_text(encoding="utf-8")
    signature = re.search(r"(?m)^func\s+\([^\n)]*\*Store[^\n)]*\)\s+Browse\s*\(", text)
    if signature:
        write(path, remove_go_function(text, signature))


def sync_docs() -> None:
    replacements = {
        "DoneSink": "completion callback",
        "RunDone": "run completion callback",
        "AssistantIteration": "assistant-iteration callback",
        "LearningToolExecuted": "learning-tool callback",
    }
    for root in (ROOT / "docs" / "contracts", ROOT / "docs" / "plans" / "active"):
        if not root.exists():
            continue
        for path in root.rglob("*.md"):
            text = path.read_text(encoding="utf-8")
            updated = text
            for old, new in replacements.items():
                updated = updated.replace(old, new)
            if updated != text:
                write(path, updated)


def wails_surface_test() -> None:
    method_re = re.compile(r"(?m)^func\s+\([^\n)]*\*?App[^\n)]*\)\s+([A-Z][A-Za-z0-9_]*)\s*\(")
    methods: set[str] = set()
    for path in ROOT.glob("*.go"):
        if not path.name.endswith("_test.go"):
            methods.update(method_re.findall(path.read_text(encoding="utf-8")))
    leaked = sorted(methods & {"RunDone", "AssistantIteration", "LearningToolExecuted"})
    if leaked or not methods:
        raise RuntimeError(f"invalid Wails surface; leaked={leaked}, methods={len(methods)}")
    literals = "\n".join(f'\t\t"{name}",' for name in sorted(methods))
    write(
        ROOT / "wails_surface_test.go",
        f'''package main

import (
    "reflect"
    "slices"
    "testing"
)

func TestWailsBindingSurface(t *testing.T) {{
    t.Parallel()
    appType := reflect.TypeOf((*App)(nil))
    got := make([]string, 0, appType.NumMethod())
    for i := 0; i < appType.NumMethod(); i++ {{
        got = append(got, appType.Method(i).Name)
    }}
    want := []string{{
{literals}
    }}
    if !slices.Equal(got, want) {{
        t.Fatalf("exported App methods changed: got %v, want %v", got, want)
    }}
}}
''',
    )


def architecture_test() -> None:
    write(
        ROOT / "architecture_invariants_test.go",
        r'''package main

import (
    "bufio"
    "io/fs"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "testing"
)

func TestBackendArchitectureInvariants(t *testing.T) {
    t.Parallel()
    _, thisFile, _, ok := runtime.Caller(0)
    if !ok {
        t.Fatal("resolve repository root")
    }
    root := filepath.Dir(thisFile)
    forbidden := strings.Join([]string{"third_party", "crush", "internal"}, "/") + "/"
    err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
        if walkErr != nil {
            return walkErr
        }
        if entry.IsDir() {
            switch entry.Name() {
            case ".git", "frontend", "build", "third_party", "node_modules":
                if path != root {
                    return filepath.SkipDir
                }
            }
            return nil
        }
        if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
            return nil
        }
        rel, err := filepath.Rel(root, path)
        if err != nil {
            return err
        }
        if rel != filepath.Base(rel) &&
            !strings.HasPrefix(rel, "internal"+string(filepath.Separator)) &&
            !strings.HasPrefix(rel, "cmd"+string(filepath.Separator)) {
            return nil
        }
        file, err := os.Open(path)
        if err != nil {
            return err
        }
        scanner := bufio.NewScanner(file)
        lines := 0
        for scanner.Scan() {
            lines++
            if strings.Contains(scanner.Text(), forbidden) {
                t.Errorf("%s crosses the Crush internal boundary", rel)
            }
        }
        scanErr := scanner.Err()
        closeErr := file.Close()
        if scanErr != nil {
            return scanErr
        }
        if closeErr != nil {
            return closeErr
        }
        if lines >= 1000 {
            t.Errorf("%s has %d lines; implementation files must stay below 1000", rel, lines)
        }
        return nil
    })
    if err != nil {
        t.Fatal(err)
    }
}
''',
    )


def patch_ci() -> None:
    root = ROOT / ".github" / "workflows"
    marker = "Generate Wails bindings before frontend checks"
    if not root.exists():
        return
    for path in sorted(root.glob("*.y*ml")):
        if path.name.startswith("agent"):
            continue
        text = path.read_text(encoding="utf-8")
        if marker in text or not re.search(r"\bnpm\s+run\s+check\b", text):
            continue
        lines = text.splitlines()
        target = next(i for i, line in enumerate(lines) if re.search(r"\bnpm\s+run\s+check\b", line))
        start = target
        while start > 0 and not re.match(r"^\s*-\s+(name|run|uses):", lines[start]):
            start -= 1
        indent_match = re.match(r"^(\s*)-", lines[start])
        if not indent_match:
            raise RuntimeError(f"cannot patch {path}")
        indent = indent_match.group(1)
        block = [
            f"{indent}- name: Set up Go for Wails bindings",
            f"{indent}  uses: actions/setup-go@v6",
            f"{indent}  with:",
            f"{indent}    go-version-file: go.mod",
            f"{indent}    cache: false",
            f"{indent}- name: {marker}",
            f"{indent}  shell: bash",
            f"{indent}  working-directory: ${{{{ github.workspace }}}}",
            f"{indent}  run: |",
            f"{indent}    version=\"$(go list -m -f '{{{{.Version}}}}' github.com/wailsapp/wails/v2)\"",
            f"{indent}    go install \"github.com/wailsapp/wails/v2/cmd/wails@${{version}}\"",
            f"{indent}    wails generate module",
        ]
        lines[start:start] = block
        write(path, "\n".join(lines))


def main() -> None:
    split_assets()
    hide_callbacks()
    portable_basenames()
    remove_dead_browse()
    sync_docs()
    wails_surface_test()
    architecture_test()
    patch_ci()


if __name__ == "__main__":
    main()

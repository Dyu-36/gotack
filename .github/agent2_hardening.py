from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    text = text.strip("\n") + "\n"
    if not path.exists() or path.read_text(encoding="utf-8") != text:
        path.write_text(text, encoding="utf-8")


def remove_import(text: str, import_path: str, qualifier: str) -> str:
    if qualifier in text:
        return text
    text = re.sub(rf"(?m)^\s*{re.escape(import_path)}\s*\n", "", text)
    return re.sub(rf"(?m)^import\s+{re.escape(import_path)}\s*\n", "", text)


def split_frontend_assets() -> None:
    directive = re.compile(
        r"(?m)^//go:embed[^\n]*frontend/dist[^\n]*\n"
        r"var\s+([A-Za-z_]\w*)\s+embed\.FS\s*\n?"
    )
    variable: str | None = None
    production: list[Path] = []

    for path in sorted(ROOT.glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        text = path.read_text(encoding="utf-8")
        match = directive.search(text)
        if not match:
            continue
        variable = variable or match.group(1)
        if re.search(r"(?m)^//go:build\s+production\s*$", text):
            production.append(path)
            continue
        updated = directive.sub("", text, count=1)
        write(path, remove_import(updated, '"embed"', "embed."))

    for path in sorted(ROOT.glob("frontend_assets_*.go")):
        text = path.read_text(encoding="utf-8")
        if re.search(r"(?m)^//go:build\s+production\s*$", text) and "frontend/dist" in text:
            production.append(path)
            match = re.search(r"(?m)^var\s+([A-Za-z_]\w*)\s+embed\.FS\s*$", text)
            if match:
                variable = variable or match.group(1)

    if variable is None:
        for path in sorted(ROOT.glob("*.go")):
            match = re.search(
                r"AssetServer:\s*&assetserver\.Options\{\s*Assets:\s*([A-Za-z_]\w*)",
                path.read_text(encoding="utf-8"),
            )
            if match:
                variable = match.group(1)
                break
    if variable is None:
        raise RuntimeError("cannot resolve the frontend asset variable")

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

    tooling = []
    for path in sorted(ROOT.glob("frontend_assets_*.go")):
        text = path.read_text(encoding="utf-8")
        if re.search(r"(?m)^//go:build\s+!production\s*$", text) and re.search(
            rf"(?m)^var\s+{re.escape(variable)}\b", text
        ):
            tooling.append(path)
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


def hide_internal_callbacks() -> None:
    renames = {
        "RunDone": "runDone",
        "AssistantIteration": "assistantIteration",
        "LearningToolExecuted": "learningToolExecuted",
    }
    for path in sorted(ROOT.glob("*.go")):
        text = path.read_text(encoding="utf-8")
        updated = text
        for old, new in renames.items():
            updated = re.sub(rf"\b{old}\b", new, updated)
        if updated != text:
            write(path, updated)


def package_name(directory: Path) -> str:
    for path in sorted(directory.glob("*.go")):
        match = re.search(r"(?m)^package\s+([A-Za-z_]\w*)\s*$", path.read_text(encoding="utf-8"))
        if match:
            return match.group(1)
    raise RuntimeError(f"cannot resolve package in {directory}")


def normalize_attachment_names() -> None:
    directories = [ROOT, ROOT / "internal" / "attachments"]
    keywords = ("attachment", "Attachment", "displayName", "DisplayName", "fileName", "FileName")

    for directory in directories:
        if not directory.exists():
            continue
        changed = False
        helper = directory / "portable_path.go"
        for path in sorted(directory.glob("*.go")):
            if path.name.endswith("_test.go") or path == helper:
                continue
            text = path.read_text(encoding="utf-8")
            if "filepath.Base(" not in text or not any(keyword in text for keyword in keywords):
                continue
            updated = text.replace("filepath.Base(", "portableBase(")
            updated = remove_import(updated, '"path/filepath"', "filepath.")
            write(path, updated)
            changed = True

        helper_referenced = changed or any(
            "portableBase(" in path.read_text(encoding="utf-8")
            for path in directory.glob("*.go")
            if path != helper
        )
        if not helper_referenced:
            continue

        package = package_name(directory)
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
    tests := map[string]string{{
        "windows": `C:\\tmp\\photo.png`,
        "unix":    "/tmp/photo.png",
        "plain":   "photo.png",
    }}
    for name, input := range tests {{
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


def remove_function(text: str, signature: re.Match[str]) -> str:
    start = signature.start()
    brace = text.find("{", signature.end())
    if brace < 0:
        raise RuntimeError("function body not found")

    depth = 0
    state = "code"
    escaped = False
    i = brace
    while i < len(text):
        char = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ""
        if state == "code":
            if char == "/" and nxt == "/":
                state = "line"
                i += 1
            elif char == "/" and nxt == "*":
                state = "block"
                i += 1
            elif char == '"':
                state = "string"
                escaped = False
            elif char == "`":
                state = "raw"
            elif char == "'":
                state = "rune"
                escaped = False
            elif char == "{":
                depth += 1
            elif char == "}":
                depth -= 1
                if depth == 0:
                    end = i + 1
                    while end < len(text) and text[end] in " \t":
                        end += 1
                    while end < len(text) and text[end] == "\n":
                        end += 1
                    return text[:start] + text[end:]
        elif state == "line":
            if char == "\n":
                state = "code"
        elif state == "block":
            if char == "*" and nxt == "/":
                state = "code"
                i += 1
        elif state in ("string", "rune"):
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif (state == "string" and char == '"') or (state == "rune" and char == "'"):
                state = "code"
        elif state == "raw" and char == "`":
            state = "code"
        i += 1
    raise RuntimeError("unterminated function body")


def remove_dead_recall_api() -> None:
    path = ROOT / "internal" / "recall" / "session.go"
    if not path.exists():
        return
    text = path.read_text(encoding="utf-8")
    signature = re.search(r"(?m)^func\s+\([^\n)]*\*Store[^\n)]*\)\s+Browse\s*\(", text)
    if signature:
        write(path, remove_function(text, signature))


def synchronize_docs() -> None:
    replacements = {
        "DoneSink": "completion callback",
        "RunDone": "run completion callback",
        "AssistantIteration": "assistant-iteration callback",
        "LearningToolExecuted": "learning-tool callback",
    }
    for directory in (ROOT / "docs" / "contracts", ROOT / "docs" / "plans" / "active"):
        if not directory.exists():
            continue
        for path in directory.rglob("*.md"):
            text = path.read_text(encoding="utf-8")
            updated = text
            for old, new in replacements.items():
                updated = updated.replace(old, new)
            if updated != text:
                write(path, updated)


def generate_wails_surface_test() -> None:
    method_re = re.compile(
        r"(?m)^func\s+\([^\n)]*\*?App[^\n)]*\)\s+([A-Z][A-Za-z0-9_]*)\s*\("
    )
    methods: set[str] = set()
    for path in sorted(ROOT.glob("*.go")):
        if not path.name.endswith("_test.go"):
            methods.update(method_re.findall(path.read_text(encoding="utf-8")))
    forbidden = {"RunDone", "AssistantIteration", "LearningToolExecuted"}
    leaked = sorted(methods & forbidden)
    if leaked:
        raise RuntimeError(f"internal callbacks remain exported: {leaked}")
    if not methods:
        raise RuntimeError("no exported App methods found")
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


def generate_architecture_test() -> None:
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


def patch_ci_ordering() -> None:
    workflow_root = ROOT / ".github" / "workflows"
    marker = "Generate Wails bindings before frontend checks"
    if not workflow_root.exists():
        return
    for path in sorted(workflow_root.glob("*.y*ml")):
        if path.name.startswith("agent2-") or path.name.startswith("agent-"):
            continue
        text = path.read_text(encoding="utf-8")
        if marker in text or not re.search(r"\bnpm\s+run\s+check\b", text):
            continue
        lines = text.splitlines()
        target = next(i for i, line in enumerate(lines) if re.search(r"\bnpm\s+run\s+check\b", line))
        step_start = target
        while step_start > 0 and not re.match(r"^\s*-\s+(name|run|uses):", lines[step_start]):
            step_start -= 1
        indent_match = re.match(r"^(\s*)-", lines[step_start])
        if not indent_match:
            raise RuntimeError(f"cannot place Wails generation step in {path}")
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
        lines[step_start:step_start] = block
        write(path, "\n".join(lines))


def main() -> None:
    split_frontend_assets()
    hide_internal_callbacks()
    normalize_attachment_names()
    remove_dead_recall_api()
    synchronize_docs()
    generate_wails_surface_test()
    generate_architecture_test()
    patch_ci_ordering()


if __name__ == "__main__":
    main()

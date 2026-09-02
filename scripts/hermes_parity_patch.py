from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{label}: expected one match, found {count}")
    return text.replace(old, new, 1)


# Detached reflection sessions cannot reuse Hermes' in-process warm cache. Keep
# only the explicit bounded projection required by the Gotack adapter contract.
prompt_path = Path("internal/reflection/prompt.go")
prompt = prompt_path.read_text(encoding="utf-8")
const_start = prompt.index("const (\n\tdigestTail")
const_end_marker = "\n)\n\ntype Message"
const_end = prompt.index(const_end_marker, const_start) + len("\n)")
new_constants = '''const (
    digestTail             = 24
    messagePreviewRunes    = 300
    toolResultPreviewRunes = 200
)'''
prompt = prompt[:const_start] + new_constants + prompt[const_end:]
tail_start = prompt.index("// Digest is Hermes'")
new_tail = '''// Digest projects only the newest transcript items into a detached review.
// Unlike Hermes' same-process warm-cache fork, Gotack creates a fresh REST
// session, so every retained item must be explicitly bounded.
func Digest(messages []Message) string {
    if len(messages) == 0 {
        return "[Conversation snapshot is empty.]"
    }
    if len(messages) > digestTail {
        messages = messages[len(messages)-digestTail:]
    }

    var out strings.Builder
    fmt.Fprintf(&out, "[Recent conversation: %d items]\\n", len(messages))
    for _, message := range messages {
        writeDigestMessage(&out, message)
    }
    return strings.TrimSpace(out.String())
}

func writeDigestMessage(out *strings.Builder, message Message) {
    role := strings.ToUpper(strings.TrimSpace(message.Role))
    if role == "" {
        role = "UNKNOWN"
    }
    fmt.Fprintf(out, "[%s]\\n", role)

    previewLimit := messagePreviewRunes
    if strings.EqualFold(role, "TOOL") {
        previewLimit = toolResultPreviewRunes
    }
    if text := oneLine(message.Text); text != "" {
        out.WriteString(truncateRunes(text, previewLimit))
        out.WriteByte('\\n')
    }
    writeTools(out, message.Tools)
    writeResults(out, message.Results)
}

func writeTools(out *strings.Builder, tools []string) {
    clean := make([]string, 0, len(tools))
    for _, tool := range tools {
        if tool = strings.TrimSpace(tool); tool != "" {
            clean = append(clean, tool)
        }
    }
    if len(clean) > 0 {
        joined := truncateRunes(strings.Join(clean, ", "), messagePreviewRunes)
        fmt.Fprintf(out, "ASSISTANT[tools: %s]\\n", joined)
    }
}

func writeResults(out *strings.Builder, results []ToolResult) {
    for _, result := range results {
        name := strings.TrimSpace(result.Name)
        if name == "" {
            name = "unknown"
        }
        status := ""
        if result.IsError {
            status = " error"
        }
        content := truncateRunes(oneLine(result.Content), toolResultPreviewRunes)
        fmt.Fprintf(out, "TOOL[%s%s]:", name, status)
        if content != "" {
            fmt.Fprintf(out, " %s", content)
        }
        out.WriteByte('\\n')
    }
}

func oneLine(text string) string { return strings.Join(strings.Fields(text), " ") }

const truncationMarker = "…[truncated]"

func truncateRunes(text string, limit int) string {
    if limit <= 0 {
        return ""
    }
    runes := []rune(text)
    if len(runes) <= limit {
        return text
    }
    marker := []rune(truncationMarker)
    if limit <= len(marker) {
        return string(runes[:limit])
    }
    return string(runes[:limit-len(marker)]) + truncationMarker
}

func runeLen(text string) int { return len([]rune(text)) }
'''
prompt_path.write_text(prompt[:tail_start] + new_tail, encoding="utf-8")


# Replace warm-cache-oriented digest assertions with strict detached-session
# retention and rune-boundary tests.
reflection_test_path = Path("internal/reflection/reflection_test.go")
reflection_test = reflection_test_path.read_text(encoding="utf-8")
reflection_test = replace_once(
    reflection_test,
    '\t"testing"\n)',
    '\t"testing"\n\t"unicode/utf8"\n)',
    "reflection test import",
)
digest_start = reflection_test.index("func TestDigestBoundsOldTextAndKeepsRecentToolPair")
digest_end = reflection_test.index(
    "func TestCombinedPromptDoesNotStopBeforeSkillOrMemoryReview", digest_start
)
new_digest_tests = '''func TestDigestKeepsOnlyLatestTwentyFourItems(t *testing.T) {
    messages := make([]Message, digestTail+2)
    for index := range messages {
        messages[index] = Message{Role: "user", Text: fmt.Sprintf("message-%02d", index)}
    }

    digest := Digest(messages)
    if strings.Contains(digest, "message-00") || strings.Contains(digest, "message-01") {
        t.Fatalf("digest retained items older than the latest %d: %s", digestTail, digest)
    }
    if !strings.Contains(digest, "message-02") || !strings.Contains(digest, "message-25") {
        t.Fatalf("digest dropped a retained boundary item: %s", digest)
    }
    if got := strings.Count(digest, "[USER]\\n"); got != digestTail {
        t.Fatalf("digest item count = %d, want %d", got, digestTail)
    }
}

func TestDigestBoundsMessageAndToolPreviewsByRunes(t *testing.T) {
    digest := Digest([]Message{
        {Role: "user", Text: strings.Repeat("界", messagePreviewRunes+25) + "USER-END"},
        {Role: "assistant", Text: strings.Repeat("答", messagePreviewRunes+25) + "ASSISTANT-END", Tools: []string{"read"}},
        {Role: "tool", Results: []ToolResult{{Name: "read", Content: strings.Repeat("工", toolResultPreviewRunes+25) + "TOOL-END"}}},
    })

    userPreview := digestLineAfter(t, digest, "[USER]")
    assistantPreview := digestLineAfter(t, digest, "[ASSISTANT]")
    toolPreview := strings.TrimPrefix(
        digestLineWithPrefix(t, digest, "TOOL[read]: "),
        "TOOL[read]: ",
    )
    for label, value := range map[string]string{
        "user": userPreview, "assistant": assistantPreview,
    } {
        if got := runeLen(value); got != messagePreviewRunes {
            t.Fatalf("%s preview runes = %d, want %d", label, got, messagePreviewRunes)
        }
    }
    if got := runeLen(toolPreview); got != toolResultPreviewRunes {
        t.Fatalf("tool preview runes = %d, want %d", got, toolResultPreviewRunes)
    }
    if strings.Contains(digest, "USER-END") || strings.Contains(digest, "ASSISTANT-END") || strings.Contains(digest, "TOOL-END") {
        t.Fatalf("digest leaked content beyond a preview boundary: %s", digest)
    }
    if strings.Count(digest, truncationMarker) != 3 || !utf8.ValidString(digest) {
        t.Fatalf("digest truncation was not explicit and rune-safe: %s", digest)
    }
}

func digestLineAfter(t *testing.T, digest, header string) string {
    t.Helper()
    lines := strings.Split(digest, "\\n")
    for index, line := range lines {
        if line == header && index+1 < len(lines) {
            return lines[index+1]
        }
    }
    t.Fatalf("missing digest header %q in %s", header, digest)
    return ""
}

func digestLineWithPrefix(t *testing.T, digest, prefix string) string {
    t.Helper()
    for _, line := range strings.Split(digest, "\\n") {
        if strings.HasPrefix(line, prefix) {
            return line
        }
    }
    t.Fatalf("missing digest line prefix %q in %s", prefix, digest)
    return ""
}

'''
reflection_test_path.write_text(
    reflection_test[:digest_start] + new_digest_tests + reflection_test[digest_end:],
    encoding="utf-8",
)


# Adopt the contract filename while preserving existing agent-owned provenance.
filesystem_path = Path("internal/skillmanage/filesystem.go")
filesystem = filesystem_path.read_text(encoding="utf-8")
filesystem = replace_once(
    filesystem,
    '\townershipFileName = ".gotack-agent-skills.json"\n\townershipVersion  = 1',
    '\townershipFileName       = ".ownership.json"\n\tlegacyOwnershipFileName = ".gotack-agent-skills.json"\n\townershipVersion        = 1',
    "ownership manifest constants",
)
ownership_start = filesystem.index("func (m *Manager) loadOwnership()")
ownership_end = filesystem.index("func (m *Manager) saveOwnership", ownership_start)
new_ownership_loader = '''func (m *Manager) loadOwnership() (map[string]bool, error) {
    data, err := m.readOwnershipManifest()
    if err != nil {
        return nil, err
    }
    if data == nil {
        return make(map[string]bool), nil
    }
    var manifest ownershipManifest
    decoder := json.NewDecoder(bytes.NewReader(data))
    decoder.DisallowUnknownFields()
    if err := decoder.Decode(&manifest); err != nil {
        return nil, fmt.Errorf("decode skill ownership: %w", err)
    }
    if manifest.Version != ownershipVersion {
        return nil, fmt.Errorf("unsupported skill ownership version %d", manifest.Version)
    }
    owned := make(map[string]bool, len(manifest.AgentOwned))
    for _, name := range manifest.AgentOwned {
        if err := validateName(name); err != nil {
            return nil, fmt.Errorf("invalid skill ownership entry: %w", err)
        }
        owned[name] = true
    }
    return owned, nil
}

// readOwnershipManifest prefers the current protected filename. The legacy
// filename is consulted only when the current file is absent, so a corrupt or
// redirected current manifest always fails closed.
func (m *Manager) readOwnershipManifest() ([]byte, error) {
    for _, fileName := range []string{ownershipFileName, legacyOwnershipFileName} {
        path := filepath.Join(m.root, fileName)
        data, err := m.readRegularFile(path)
        if errors.Is(rootCause(err), fs.ErrNotExist) {
            continue
        }
        if err != nil {
            return nil, fmt.Errorf("read skill ownership %s: %w", fileName, err)
        }
        return data, nil
    }
    return nil, nil
}

'''
filesystem_path.write_text(
    filesystem[:ownership_start] + new_ownership_loader + filesystem[ownership_end:],
    encoding="utf-8",
)


manager_test_path = Path("internal/skillmanage/manager_test.go")
manager_test = manager_test_path.read_text(encoding="utf-8")
test_insert = manager_test.index("func TestUnreadableOwnershipFailsClosed")
migration_test = '''func TestLegacyOwnershipManifestMigratesOnNextOwnershipWrite(t *testing.T) {
    manager := newTestManager(t)
    mustApply(t, manager, Operation{
        Action: actionCreate, Name: "legacy-owned",
        Content: skillText("legacy-owned", "Use when testing legacy ownership.", "Original."),
    })
    legacy := "{\\\"version\\\":1,\\\"agent_owned\\\":[\\\"legacy-owned\\\"]}\\n"
    if err := os.WriteFile(filepath.Join(manager.Root(), legacyOwnershipFileName), []byte(legacy), 0o644); err != nil {
        t.Fatal(err)
    }

    review := RequestMeta{SessionID: "review-migrate", BackgroundReview: true}
    mustView(t, manager, "legacy-owned", "", review)
    replacement := "Updated."
    if result := manager.ApplyWithMeta(context.Background(), []Operation{{
        Action: actionPatch, Name: "legacy-owned", OldString: "Original.", NewString: &replacement,
    }}, review); !result.Success {
        t.Fatalf("legacy ownership was not honored: %+v", result)
    }
    if result := manager.ApplyWithMeta(context.Background(), []Operation{{
        Action: actionCreate, Name: "new-owned",
        Content: skillText("new-owned", "Use when testing ownership migration.", "Run it."),
    }}, review); !result.Success {
        t.Fatalf("ownership migration write failed: %+v", result)
    }
    if _, err := os.Stat(filepath.Join(manager.Root(), ownershipFileName)); err != nil {
        t.Fatalf("new ownership manifest missing: %v", err)
    }
    owned, err := manager.loadOwnership()
    if err != nil {
        t.Fatal(err)
    }
    if !owned["legacy-owned"] || !owned["new-owned"] {
        t.Fatalf("migrated ownership = %v", owned)
    }
}

'''
manager_test_path.write_text(
    manager_test[:test_insert] + migration_test + manager_test[test_insert:],
    encoding="utf-8",
)


# Make the ordinary CI workflow self-sufficient on a clean checkout. Wails v2
# needs an embeddable directory while generating bindings; the real frontend
# build replaces this minimal placeholder before Go test/vet run.
ci_path = Path(".github/workflows/ci.yml")
ci = ci_path.read_text(encoding="utf-8")
go_old = '''      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: true
      - name: Test
        run: go test ./...
'''
go_new = '''      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: true
      - name: Set up pnpm
        uses: pnpm/action-setup@v4
        with:
          version: ${{ env.PNPM_VERSION }}
          run_install: false
      - name: Set up Node.js
        uses: actions/setup-node@v5
        with:
          node-version: 24
          cache: pnpm
          cache-dependency-path: pnpm-lock.yaml
      - name: Install frontend dependencies
        run: pnpm --dir frontend install --frozen-lockfile
      - name: Prepare Wails binding generation
        run: |
          mkdir -p frontend/dist
          printf '%s\\n' '<!doctype html><html><body></body></html>' > frontend/dist/index.html
      - name: Generate Wails bindings
        run: go run github.com/wailsapp/wails/v2/cmd/wails@${{ env.WAILS_VERSION }} generate module
      - name: Build frontend assets for Go embed
        run: pnpm --dir frontend build
      - name: Test learning-loop internals
        run: go test ./internal/memory ./internal/skillmanage ./internal/recall ./internal/reflection ./internal/guard
      - name: Test learning-loop helper binaries
        run: go test ./cmd/memory ./cmd/skills ./cmd/recall ./cmd/guard
      - name: Test all Go packages
        run: go test ./...
'''
ci = replace_once(ci, go_old, go_new, "Go CI setup")
ci = replace_once(
    ci,
    '''      - name: Vet
        run: go vet ./...
      # CI ran only test and vet, so two real kinds of drift could merge without
''',
    '''      - name: Vet
        run: go vet ./...
      - name: Check repository invariants
        run: node scripts/check-repository-invariants.mjs
      # CI ran only test and vet, so two real kinds of drift could merge without
''',
    "CI invariant step",
)
frontend_start = ci.index("  frontend:")
windows_start = ci.index("  windows:", frontend_start)
frontend = ci[frontend_start:windows_start]
frontend = replace_once(
    frontend,
    '''      - name: Checkout
        uses: actions/checkout@v5
      - name: Set up pnpm
''',
    '''      - name: Checkout
        uses: actions/checkout@v5
      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: true
      - name: Set up pnpm
''',
    "frontend Go setup",
)
frontend = replace_once(
    frontend,
    '''      - name: Install dependencies
        run: pnpm --dir frontend install --frozen-lockfile
      - name: Check repository invariants
''',
    '''      - name: Install dependencies
        run: pnpm --dir frontend install --frozen-lockfile
      - name: Prepare Wails binding generation
        run: |
          mkdir -p frontend/dist
          printf '%s\\n' '<!doctype html><html><body></body></html>' > frontend/dist/index.html
      - name: Generate Wails bindings
        run: go run github.com/wailsapp/wails/v2/cmd/wails@${{ env.WAILS_VERSION }} generate module
      - name: Check repository invariants
''',
    "frontend binding generation",
)
ci_path.write_text(ci[:frontend_start] + frontend + ci[windows_start:], encoding="utf-8")

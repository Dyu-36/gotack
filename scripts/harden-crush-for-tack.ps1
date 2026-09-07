param(
    [Parameter(Mandatory = $true)]
    [string]$CrushDir
)

$ErrorActionPreference = 'Stop'

function Update-ExactText {
    param(
        [Parameter(Mandatory = $true)][string]$RelativePath,
        [Parameter(Mandatory = $true)][string]$Old,
        [Parameter(Mandatory = $true)][AllowEmptyString()][string]$New
    )

    $path = Join-Path $CrushDir $RelativePath
    if (-not (Test-Path $path)) {
        throw "Required Crush source file not found: $RelativePath"
    }
    $text = [IO.File]::ReadAllText($path, [System.Text.Encoding]::UTF8)
    if ($text.Contains($Old)) {
        $text = $text.Replace($Old, $New)
        [IO.File]::WriteAllText($path, $text, (New-Object System.Text.UTF8Encoding($false)))
        return
    }
    if ($New -eq '' -or $text.Contains($New)) {
        return
    }
    throw "Expected Crush source marker not found in ${RelativePath}: $Old"
}

function Remove-SourceFile {
    param([Parameter(Mandatory = $true)][string]$RelativePath)
    $path = Join-Path $CrushDir $RelativePath
    if (Test-Path $path) {
        Remove-Item -Force $path
    }
}

# The desktop app does not expose an interactive Question surface. Strip the
# model tool itself and its coordinator dependency so the agent schema can
# never advertise or call it, even if a future frontend accidentally
# reintroduces an event listener.
Update-ExactText 'internal/agent/coordinator.go' @'
	"github.com/charmbracelet/crush/internal/question"
'@ ''
Update-ExactText 'internal/agent/coordinator.go' @'
	questions   question.Service
'@ ''
Update-ExactText 'internal/agent/coordinator.go' @'
	Questions   question.Service
'@ ''
Update-ExactText 'internal/agent/coordinator.go' @'
		questions:    opts.Questions,
'@ ''
Update-ExactText 'internal/agent/coordinator.go' @'
	// Question tool is interactive-only and not available to sub-agents.
	if !isSubAgent && c.interactive {
		allTools = append(allTools, tools.NewQuestionTool(c.questions))
	}

'@ ''
Update-ExactText 'internal/app/app.go' @'
		Questions:   app.Questions,
'@ ''
Update-ExactText 'internal/config/config.go' @'
		"question",
'@ ''
Update-ExactText 'internal/ui/chat/tools.go' @'
	case tools.QuestionToolName:
		item = NewQuestionToolMessageItem(sty, toolCall, result, canceled)
'@ ''

Remove-SourceFile 'internal/agent/tools/question.go'
Remove-SourceFile 'internal/agent/tools/question.md'
Remove-SourceFile 'internal/agent/tools/question_test.go'
Remove-SourceFile 'internal/ui/chat/question.go'

# The isolated E2E lane blocks all fallback internet. Give the generated engine
# an explicit opt-out for its asynchronous release check so the harness can
# distinguish provider traffic from accidental egress without changing normal
# product behavior.
Update-ExactText 'internal/app/app.go' @'
	// Check for updates in the background.
	go app.checkForUpdates(ctx)
'@ @'
	// Check for updates in the background unless an isolated host explicitly
	// disables network release checks.
	if os.Getenv("CRUSH_DISABLE_UPDATE_CHECK") != "1" {
		go app.checkForUpdates(ctx)
	}
'@

# Remove the headless REST entry points as a second boundary. The remaining
# upstream TUI question service is not registered as an agent dependency and is
# not reachable through Gotack's server contract.
Update-ExactText 'internal/server/server.go' @'
	mux.HandleFunc("POST /v1/workspaces/{id}/questions/answer", c.handlePostWorkspaceQuestionsAnswer)
'@ ''
Update-ExactText 'internal/server/server.go' @'
	mux.HandleFunc("POST /v1/workspaces/{id}/questions/cancel", c.handlePostWorkspaceQuestionsCancel)
'@ ''

# The two route removals above intentionally make the generated HTTP handlers
# unreachable. Remove the handler definitions in the same hardening layer so
# Staticcheck does not report them as orphaned U1000 symbols. Keep the upstream
# question service itself intact for the upstream TUI path.
$protoPath = Join-Path $CrushDir 'internal/server/proto.go'
$protoText = [IO.File]::ReadAllText($protoPath, [System.Text.Encoding]::UTF8)
$questionHandlerStart = '// handlePostWorkspaceQuestionsAnswer submits answers for a batch question.'
$nextHandlerStart = '// handlePostWorkspacePermissionsSkip sets whether to skip permission prompts.'
$questionHandlerIndex = $protoText.IndexOf($questionHandlerStart, [StringComparison]::Ordinal)
if ($questionHandlerIndex -ge 0) {
    $nextHandlerIndex = $protoText.IndexOf($nextHandlerStart, $questionHandlerIndex, [StringComparison]::Ordinal)
    if ($nextHandlerIndex -lt 0) {
        throw 'Question REST handler boundary marker missing in internal/server/proto.go.'
    }
    $protoText = $protoText.Remove($questionHandlerIndex, $nextHandlerIndex - $questionHandlerIndex)
    [IO.File]::WriteAllText($protoPath, $protoText, (New-Object System.Text.UTF8Encoding($false)))
}
elseif ($protoText.Contains('handlePostWorkspaceQuestionsAnswer') -or $protoText.Contains('handlePostWorkspaceQuestionsCancel')) {
    throw 'Question REST handlers changed shape; hardening refused a partial removal.'
}

# Keep the clean replay compatible with the repository's stricter standalone
# Staticcheck gate. Remove genuinely dead upstream helpers. For the few
# deliberate upstream exceptions already documented by nolint/golangci rules,
# add native Staticcheck directives so `staticcheck ./...` and golangci agree
# without changing user-facing copy, MCP logging, or process-group isolation.
Update-ExactText 'internal/agent/tools/mcp/channel.go' @'
// isOpen reports whether the gate has been resolved to open.
func (g *channelGate) isOpen() bool {
	return channelGateState(g.state.Load()) == stateGateOpen
}

'@ ''
Update-ExactText 'internal/agent/tools/mcp/init.go' @'
	opts := &mcp.ClientOptions{
		LoggingMessageHandler: func(ctx context.Context, req *mcp.LoggingMessageRequest) {
'@ @'
	opts := &mcp.ClientOptions{
		//lint:ignore SA1019 MCP logging is intentionally retained during the protocol deprecation window.
		LoggingMessageHandler: func(ctx context.Context, req *mcp.LoggingMessageRequest) {
'@
Update-ExactText 'internal/cmd/root.go' @'
	_ "embed"
'@ ''
Update-ExactText 'internal/cmd/root.go' @'
			return errors.New("Crush crashed. If metrics are enabled, we were notified about it. If you'd like to report it, please copy the stacktrace above and open an issue at https://github.com/charmbracelet/crush/issues/new?template=bug.yml") //nolint:staticcheck
'@ @'
			//lint:ignore ST1005 This user-facing sentence intentionally starts with the product name.
			return errors.New("Crush crashed. If metrics are enabled, we were notified about it. If you'd like to report it, please copy the stacktrace above and open an issue at https://github.com/charmbracelet/crush/issues/new?template=bug.yml")
'@
Update-ExactText 'internal/cmd/root.go' @'
func createDotCrushDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create data directory: %q %w", dir, err)
	}

	gitIgnorePath := filepath.Join(dir, ".gitignore")
	content, err := os.ReadFile(gitIgnorePath)

	// create or update if old version
	if os.IsNotExist(err) || string(content) == oldGitIgnore {
		if err := os.WriteFile(gitIgnorePath, []byte(defaultGitIgnore), 0o644); err != nil {
			return fmt.Errorf("failed to create .gitignore file: %q %w", gitIgnorePath, err)
		}
	}

	return nil
}

//go:embed gitignore/old
var oldGitIgnore string

//go:embed gitignore/default
var defaultGitIgnore string
'@ ''
Update-ExactText 'internal/config/provider.go' @'
				catwalkErr = fmt.Errorf("Crush was unable to fetch an updated list of providers from %s. Consider setting CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1 to use the embedded providers bundled at the time of this Crush release. You can also update providers manually. For more info see crush update-providers --help.\n\nCause: %w", catwalkURL, err) //nolint:staticcheck
'@ @'
				//lint:ignore ST1005 This user-facing sentence intentionally starts with the product name.
				catwalkErr = fmt.Errorf("Crush was unable to fetch an updated list of providers from %s. Consider setting CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1 to use the embedded providers bundled at the time of this Crush release. You can also update providers manually. For more info see crush update-providers --help.\n\nCause: %w", catwalkURL, err)
'@
Update-ExactText 'internal/config/provider.go' @'
				hyperErr = fmt.Errorf("Crush was unable to fetch updated information from Hyper: %w", err) //nolint:staticcheck
'@ @'
				//lint:ignore ST1005 This user-facing sentence intentionally starts with the product name.
				hyperErr = fmt.Errorf("Crush was unable to fetch updated information from Hyper: %w", err)
'@
Update-ExactText 'internal/shell/run.go' @'
	// group isolation, so we use the deprecated ExecHandler instead.
	return interp.ExecHandler(handler)
'@ @'
	// group isolation, so we use the deprecated ExecHandler instead.
	//lint:ignore SA1019 ExecHandlers would append DefaultExecHandler and break required process-group isolation.
	return interp.ExecHandler(handler)
'@
Update-ExactText 'internal/ui/dialog/arguments.go' @'
	const scrollbarWidth = 1
'@ ''
Update-ExactText 'internal/ui/dialog/models_list.go' @'
type modelGroups []ModelGroup

func (m modelGroups) Len() int {
	n := 0
	for _, g := range m {
		n += len(g.Items)
	}
	return n
}

func (m modelGroups) String(i int) string {
	count := 0
	for _, g := range m {
		if i < count+len(g.Items) {
			return g.Items[i-count].Filter()
		}
		count += len(g.Items)
	}
	return ""
}
'@ ''
Update-ExactText 'internal/ui/dialog/oauth.go' @'
	cancelFunc      context.CancelFunc
'@ ''
Update-ExactText 'internal/ui/dialog/permissions.go' @'
	windowWidth  int // Terminal window dimensions.
	windowHeight int
'@ ''
Update-ExactText 'internal/ui/diffview/diffview_test.go' @'
func assertHeight(t *testing.T, expected int, output string) {
	output = strings.TrimSuffix(output, "\n")
	lines := strings.Count(output, "\n") + 1
	if lines != expected {
		t.Errorf("expected output height to be == %d, got %d", expected, lines)
	}
}
'@ ''

# Rebrand every model-visible identity string while preserving upstream Go
# module paths, legacy executable names, crush.json, built-in skill IDs, and
# the crush:// skills URI scheme.
Update-ExactText 'internal/agent/templates/coder.md.tpl' 'You are Crush, a powerful AI Assistant that runs in the CLI.' 'You are Tack, a powerful AI Assistant that runs in the CLI.'
Update-ExactText 'internal/agent/templates/agentic_fetch_prompt.md.tpl' 'You are a web content analysis agent for Crush.' 'You are a web content analysis agent for Tack.'
Update-ExactText 'internal/agent/templates/task.md.tpl' 'You are an agent for Crush.' 'You are an agent for Tack.'

Update-ExactText 'internal/agent/tools/crush_info.go' 'const CrushInfoToolName = "crush_info"' 'const CrushInfoToolName = "tack_info"'
Update-ExactText 'internal/agent/tools/crush_logs.go' 'const CrushLogsToolName = "crush_logs"' 'const CrushLogsToolName = "tack_logs"'
Update-ExactText 'internal/config/config.go' '"crush_info"' '"tack_info"'
Update-ExactText 'internal/config/config.go' '"crush_logs"' '"tack_logs"'
Update-ExactText 'internal/config/config.go' 'Add Generated with Crush line to commit messages and issues and PRs' 'Add Generated with Tack line to commit messages and issues and PRs'

Update-ExactText 'internal/agent/tools/crush_info.md' "Get Crush's current runtime state" "Get Tack's current runtime state"
Update-ExactText 'internal/agent/tools/crush_logs.md.tpl' "Read Crush's internal application logs" "Read Tack's internal application logs"
Update-ExactText 'internal/agent/tools/crush_logs.md.tpl' "Returns recent log entries from Crush's internal log file" "Returns recent log entries from Tack's internal log file"
Update-ExactText 'internal/agent/tools/crush_logs.md.tpl' 'Use to diagnose issues with Crush itself' 'Use to diagnose issues with Tack itself'

$heartEmoji = [char]::ConvertFromUtf32(0x1F498)
Update-ExactText 'internal/agent/tools/bash.md.tpl' "$heartEmoji Generated with Crush" "$heartEmoji Generated with Tack"
Update-ExactText 'internal/agent/tools/bash.md.tpl' 'Assisted-by: Crush:{{ .ModelID }}' 'Assisted-by: Tack:{{ .ModelID }}'
Update-ExactText 'internal/agent/tools/bash.md.tpl' 'Co-Authored-By: Crush <crush@charm.land>' 'Co-Authored-By: Tack <tack@gotack.local>'

# Built-in skill IDs stay crush-* for compatibility, but their prose is part
# of the model-visible context and must describe this product as Tack.
Update-ExactText 'internal/skills/builtin/crush-config/SKILL.md' 'Crush' 'Tack'
Update-ExactText 'internal/skills/builtin/crush-hooks/SKILL.md' 'Crush' 'Tack'

# Rebrand workspace default data directory from .crush to .tack
Update-ExactText 'internal/config/config.go' 'defaultDataDirectory = ".crush"' 'defaultDataDirectory = ".tack"'
Update-ExactText 'internal/fsext/fileutil.go' @'
		".crush":           true,
'@ @'
		".crush":           true,
		".tack":            true,
'@
Update-ExactText 'internal/fsext/ls.go' @'
	".crush":          true,
'@ @'
	".crush":          true,
	".tack":           true,
'@
Update-ExactText 'internal/fsext/ls.go' @'
		for _, ignoreFile := range []string{".gitignore", ".crushignore"} {
'@ @'
		for _, ignoreFile := range []string{".gitignore", ".crushignore", ".tackignore"} {
'@
Update-ExactText 'internal/commands/commands.go' @'
		{
			path:   filepath.Join(home.Dir(), ".crush", "commands"),
			prefix: userCommandPrefix,
		},
'@ @'
		{
			path:   filepath.Join(home.Dir(), ".tack", "commands"),
			prefix: userCommandPrefix,
		},
		{
			path:   filepath.Join(home.Dir(), ".crush", "commands"),
			prefix: userCommandPrefix,
		},
'@
Update-ExactText 'internal/cmd/stats.go' @'
	if outputDataDir == "" {
		outputDataDir = ".crush"
	}
'@ @'
	if outputDataDir == "" {
		outputDataDir = ".tack"
	}
'@
Update-ExactText 'internal/cmd/stats.go' @'
			if filepath.Base(dir) == ".crush" {
'@ @'
			if filepath.Base(dir) == ".tack" || filepath.Base(dir) == ".crush" {
'@

Write-Host 'Stripped Question surfaces, applied Tack identity/data directory, and cleaned standalone Staticcheck findings.'

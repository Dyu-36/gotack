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
    $text = Get-Content $path -Raw
    if ($text.Contains($Old)) {
        $text = $text.Replace($Old, $New)
        Set-Content -Path $path -Value $text -NoNewline
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

Remove-SourceFile 'internal/agent/tools/question.go'
Remove-SourceFile 'internal/agent/tools/question.md'
Remove-SourceFile 'internal/agent/tools/question_test.go'

# Remove the headless REST entry points as a second boundary. The remaining
# upstream TUI question service is not registered as an agent dependency and is
# not reachable through Gotack's server contract.
Update-ExactText 'internal/server/server.go' @'
	mux.HandleFunc("POST /v1/workspaces/{id}/questions/answer", c.handlePostWorkspaceQuestionsAnswer)
'@ ''
Update-ExactText 'internal/server/server.go' @'
	mux.HandleFunc("POST /v1/workspaces/{id}/questions/cancel", c.handlePostWorkspaceQuestionsCancel)
'@ ''

# Rebrand every model-visible identity string while preserving upstream Go
# module paths, legacy executable names, and the crush:// skills URI scheme.
Update-ExactText 'internal/agent/templates/coder.md.tpl' 'You are Crush, a powerful AI Assistant that runs in the CLI.' 'You are Tack, a powerful AI Assistant that runs in the CLI.'
Update-ExactText 'internal/agent/templates/agentic_fetch_prompt.md.tpl' 'You are a web content analysis agent for Crush.' 'You are a web content analysis agent for Tack.'
Update-ExactText 'internal/agent/templates/task.md.tpl' 'You are an agent for Crush.' 'You are an agent for Tack.'

Update-ExactText 'internal/agent/tools/crush_info.go' 'const CrushInfoToolName = "crush_info"' 'const CrushInfoToolName = "tack_info"'
Update-ExactText 'internal/agent/tools/crush_logs.go' 'const CrushLogsToolName = "crush_logs"' 'const CrushLogsToolName = "tack_logs"'
Update-ExactText 'internal/config/config.go' '"crush_info"' '"tack_info"'
Update-ExactText 'internal/config/config.go' '"crush_logs"' '"tack_logs"'

Update-ExactText 'internal/agent/tools/crush_info.md' "Get Crush's current runtime state" "Get Tack's current runtime state"
Update-ExactText 'internal/agent/tools/crush_logs.md.tpl' "Read Crush's internal application logs" "Read Tack's internal application logs"
Update-ExactText 'internal/agent/tools/crush_logs.md.tpl' "Returns recent log entries from Crush's internal log file" "Returns recent log entries from Tack's internal log file"
Update-ExactText 'internal/agent/tools/crush_logs.md.tpl' 'Use to diagnose issues with Crush itself' 'Use to diagnose issues with Tack itself'

Update-ExactText 'internal/agent/tools/bash.md.tpl' '💘 Generated with Crush' '💘 Generated with Tack'
Update-ExactText 'internal/agent/tools/bash.md.tpl' 'Assisted-by: Crush:{{ .ModelID }}' 'Assisted-by: Tack:{{ .ModelID }}'
Update-ExactText 'internal/agent/tools/bash.md.tpl' 'Co-Authored-By: Crush <crush@charm.land>' 'Co-Authored-By: Tack <tack@gotack.local>'

Write-Host 'Stripped the Question agent tool and applied Tack model identity.'

package runmetrics

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

func validEffort(value string) bool {
	return enum(value, "", "none", "minimal", "low", "medium", "high", "xhigh", "max")
}

func validateExecution(run *engineapi.RunTelemetry) error {
	if !idPattern.MatchString(run.SessionID) || !validEffort(run.RequestedReasoningEffort) || !validEffort(run.ResolvedReasoningEffort) {
		return errors.New("telemetry_execution_identity_invalid")
	}
	if run.StartedAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, run.StartedAt); err != nil {
			return errors.New("telemetry_start_time_invalid")
		}
	}
	for _, build := range []*engineapi.BuildTelemetry{run.EngineBuild, run.AppBuild} {
		if build == nil {
			continue
		}
		if !idPattern.MatchString(build.ID) || !idPattern.MatchString(build.Commit) || !idPattern.MatchString(build.SourceDigest) {
			return errors.New("telemetry_build_identity_invalid")
		}
		if build.BuiltAt != "" {
			if _, err := time.Parse(time.RFC3339Nano, build.BuiltAt); err != nil {
				return errors.New("telemetry_build_time_invalid")
			}
		}
	}
	if run.ExecutionRecordsDropped < 0 || len(run.ModelCalls) > 2048 || len(run.ToolCalls) > 2048 || len(run.ProviderAttempts) > 2048 {
		return errors.New("telemetry_execution_count_invalid")
	}
	ids := map[int]bool{}
	for _, call := range run.ModelCalls {
		if call.ID <= 0 || ids[call.ID] || call.Step <= 0 || call.StartedMicros < 0 || call.DeliveryMicros < 0 || !enum(call.Purpose, "tool_loop", "title", "summarize") || !labelPattern.MatchString(call.Provider) || !labelPattern.MatchString(call.Model) {
			return errors.New("telemetry_model_call_invalid")
		}
		ids[call.ID] = true
		if !validEffort(call.RequestedEffort) || !validEffort(call.ResolvedEffort) || !validEffort(call.FinalReasoning.Effort) || !enum(call.FinalReasoning.Thinking, "", "enabled", "disabled", "adaptive") {
			return errors.New("telemetry_reasoning_options_invalid")
		}
		for _, offset := range []*int64{call.EndedMicros, call.FirstStreamEventMicros, call.FirstReasoningMicros, call.ReasoningEndMicros, call.FirstToolMicros, call.ToolInputEndMicros, call.FirstTextMicros} {
			if offset != nil && (*offset < call.StartedMicros || (call.EndedMicros != nil && *offset > *call.EndedMicros)) {
				return errors.New("telemetry_model_timing_invalid")
			}
		}
		for _, count := range []*int64{call.InputTokens, call.OutputTokens, call.ReasoningTokens, call.CacheReadTokens, call.MaxOutputTokens, call.FinalReasoning.BudgetTokens} {
			if count != nil && *count < 0 {
				return errors.New("telemetry_model_usage_invalid")
			}
		}
	}
	for _, call := range run.ToolCalls {
		if call.Step <= 0 || !idPattern.MatchString(call.ID) || !labelPattern.MatchString(call.Name) || call.StartedMicros < 0 || call.EndedMicros < call.StartedMicros || call.HookMicros < 0 || call.PermissionMicros < 0 || !enum(call.Status, "", "running", "completed", "failed", "cancelled") {
			return errors.New("telemetry_tool_call_invalid")
		}
	}
	return nil
}

func cloneExecution(run *engineapi.RunTelemetry) {
	if run.AppBuild != nil {
		copied := *run.AppBuild
		run.AppBuild = &copied
	}
	if run.EngineBuild != nil {
		copied := *run.EngineBuild
		run.EngineBuild = &copied
	}
	copyJSON := func(source, destination any) {
		if encoded, err := json.Marshal(source); err == nil {
			_ = json.Unmarshal(encoded, destination)
		}
	}
	if run.ModelCalls != nil {
		var copied []engineapi.ModelCallTelemetry
		copyJSON(run.ModelCalls, &copied)
		run.ModelCalls = copied
	}
	if run.ToolCalls != nil {
		var copied []engineapi.ToolCallTelemetry
		copyJSON(run.ToolCalls, &copied)
		run.ToolCalls = copied
	}
}

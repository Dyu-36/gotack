package main

import (
	"errors"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/contextseed"
	"github.com/Dyu-36/gotack/internal/memory"
)

type AssistantContextInfo struct {
	Directory  string                  `json:"directory"`
	ProfileCap int                     `json:"profile_cap_chars"`
	MemoryCap  int                     `json:"memory_cap_chars"`
	Snapshot   contextseed.PromptStats `json:"snapshot"`
	Import     memory.ImportReport     `json:"import"`
}

// AssistantContextInfo is a read-only, local inspection; it does not call a model
// or expose prompt contents. Character/byte counts are not reported as tokens.
func (a *App) AssistantContextInfo() (AssistantContextInfo, error) {
	if a.contextSeeder == nil {
		return AssistantContextInfo{}, errors.New("assistant context is not initialized")
	}
	report, err := memory.ReadImportReport(appconfig.Dir())
	if err != nil {
		return AssistantContextInfo{}, err
	}
	return AssistantContextInfo{Directory: a.contextSeeder.ContextDir(), ProfileCap: memory.ProfileCap,
		MemoryCap: memory.MemoryCap, Snapshot: a.contextSeeder.PromptStats(), Import: report}, nil
}

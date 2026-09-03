package main

import (
	"reflect"
	"slices"
	"testing"
)

func TestWailsBindingSurfaceMatchesContract(t *testing.T) {
	appType := reflect.TypeOf(App{})
	for i := 0; i < appType.NumField(); i++ {
		field := appType.Field(i)
		if field.IsExported() {
			t.Errorf("App field %s is exported and would expand the Wails contract", field.Name)
		}
	}

	expected := []string{
		"AnswerPermission",
		"AnswerQuestion",
		"AttachmentLimits",
		"BackendReady",
		"CancelPrompt",
		"ChangedFiles",
		"CloseTerminal",
		"CreateSession",
		"CurrentWorkspace",
		"DeleteProvider",
		"DeleteSession",
		"EngineStatus",
		"EnsureAssistantWorkspace",
		"FileDiff",
		"GetChatGPTOAuthStatus",
		"GetProviderUsage",
		"GetSettings",
		"GetZaloConfig",
		"ListProviders",
		"ListRecentWorkspaces",
		"ListSessions",
		"LoginChatGPTOAuth",
		"LogoutChatGPTOAuth",
		"OpenGeneratedFile",
		"OpenTerminal",
		"OpenWorkspace",
		"PickPromptFiles",
		"ReconnectEngine",
		"RegenerateZaloPairingCode",
		"RemoveZaloToken",
		"RenameSession",
		"ResizeTerminal",
		"RevealGeneratedFile",
		"RevealProviderAPIKey",
		"SaveSettings",
		"SaveZaloConfig",
		"SelectWorkspace",
		"SendPrompt",
		"SendZaloFile",
		"SessionMessages",
		"StartEngine",
		"StopEngine",
		"SwitchSession",
		"TestZaloConnection",
		"UnpairZaloChat",
		"WriteTerminal",
		"ZaloStatus",
	}

	typeOfApp := reflect.TypeOf((*App)(nil))
	actual := make([]string, 0, typeOfApp.NumMethod())
	for i := 0; i < typeOfApp.NumMethod(); i++ {
		actual = append(actual, typeOfApp.Method(i).Name)
	}
	if !slices.Equal(actual, expected) {
		t.Fatalf("exported App methods = %v, want contract surface %v", actual, expected)
	}
}

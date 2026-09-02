package main

import (
	"reflect"
	"slices"
	"testing"
)

func TestWailsBindingSurfaceMatchesContract(t *testing.T) {
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
		"GetSettings",
		"GetZaloConfig",
		"ListProviders",
		"ListRecentWorkspaces",
		"ListSessions",
		"LoginChatGPTOAuth",
		"LogoutChatGPTOAuth",
		"OpenTerminal",
		"OpenWorkspace",
		"PickPromptFiles",
		"ReconnectEngine",
		"RegenerateZaloPairingCode",
		"RemoveZaloToken",
		"RenameSession",
		"ResizeTerminal",
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

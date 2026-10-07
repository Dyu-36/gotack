package provider

import (
	"strings"
	"testing"
)

func TestValidateChatModel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, provider, model string
		wantError             bool
	}{
		{"batch", "openrouter", "deepseek/deepseek-v4.1-flash:batch", true},
		{"batch before routing suffix", "openrouter", "vendor/model:batch:nitro", true},
		{"batch after routing suffix", "openrouter", "vendor/model:nitro:batch", true},
		{"padded IDs", " openrouter ", " vendor/model:batch ", true},
		{"base model", "openrouter", "deepseek/deepseek-v4.1-flash", false},
		{"free model", "openrouter", "vendor/model:free", false},
		{"routing suffix", "openrouter", "vendor/model:nitro", false},
		{"batch in model name", "openrouter", "vendor/batch-model", false},
		{"different suffix", "openrouter", "vendor/model:batching", false},
		{"other provider", "custom", "vendor/model:batch", false},
		{"Bedrock version", "bedrock", "amazon.nova-lite-v1:0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateChatModel(tc.provider, tc.model)
			if (err != nil) != tc.wantError {
				t.Fatalf("ValidateChatModel() = %v, wantError %v", err, tc.wantError)
			}
			if err != nil && !strings.Contains(err.Error(), "select a chat model without :batch") {
				t.Fatalf("missing recovery guidance: %v", err)
			}
		})
	}
}

func TestApplyRejectsBatchModelBeforeEngineRequests(t *testing.T) {
	t.Parallel()
	err := Apply(t.Context(), nil, "ws", Settings{Provider: "openrouter", Model: "vendor/model:batch"}, "test-key")
	if err == nil || !strings.Contains(err.Error(), "batch processing only") {
		t.Fatalf("Apply() = %v, want batch validation before accessing the engine", err)
	}
}

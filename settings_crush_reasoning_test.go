package main

import "testing"

func TestCrushReasoning(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantEffort string
		wantThink bool
	}{
		{name: "off", value: "none", wantEffort: "", wantThink: false},
		{name: "blank", value: "", wantEffort: "", wantThink: false},
		{name: "low", value: "low", wantEffort: "low", wantThink: true},
		{name: "medium with whitespace", value: " Medium ", wantEffort: "medium", wantThink: true},
		{name: "high", value: "high", wantEffort: "high", wantThink: true},
		{name: "xhigh", value: "XHIGH", wantEffort: "xhigh", wantThink: true},
		{name: "max", value: "max", wantEffort: "max", wantThink: true},
		{name: "unknown", value: "turbo", wantEffort: "", wantThink: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			effort, think := crushReasoning(tc.value)
			if effort != tc.wantEffort || think != tc.wantThink {
				t.Fatalf("crushReasoning(%q) = (%q, %v), want (%q, %v)", tc.value, effort, think, tc.wantEffort, tc.wantThink)
			}
		})
	}
}

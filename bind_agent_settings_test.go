package main

import "testing"

func TestNormalizeDisabledTools(t *testing.T) {
	got, err := normalizeDisabledTools([]string{"write", "read", "write", ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "read" || got[1] != "write" {
		t.Fatalf("unexpected disabled tools: %v", got)
	}
	if _, err := normalizeDisabledTools([]string{"not-a-tool"}); err == nil {
		t.Fatal("expected unknown tool to fail")
	}
}

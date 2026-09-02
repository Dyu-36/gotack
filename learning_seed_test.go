package main

import (
	"reflect"
	"testing"
)

func TestSkillsEntryShape(t *testing.T) {
	got := skillsEntry("C:/bin/skills.exe", "C:/data/skills")
	want := map[string]any{
		"command": "C:/bin/skills.exe",
		"args":    []string{"--root", "C:/data/skills"},
		"type":    "stdio",
		"timeout": 30,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("skillsEntry = %#v, want %#v", got, want)
	}
}

func TestRecallEntryShape(t *testing.T) {
	got := recallEntry("C:/bin/recall.exe", "C:/crush/data", "C:/gotack/recall/ws")
	want := map[string]any{
		"command": "C:/bin/recall.exe",
		"args": []string{
			"--data-dir", "C:/crush/data",
			"--index-dir", "C:/gotack/recall/ws",
		},
		"type":    "stdio",
		"timeout": 30,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recallEntry = %#v, want %#v", got, want)
	}
}

func TestMissingLearningBinariesRemoveMCPEntries(t *testing.T) {
	fake := &memoryAPI{t: t}
	app := newMemoryTestApp(t, fake)
	oldSkills, oldRecall := resolveSkillsCommand, resolveRecallCommand
	resolveSkillsCommand = func() string { return "" }
	resolveRecallCommand = func() string { return "" }
	t.Cleanup(func() {
		resolveSkillsCommand = oldSkills
		resolveRecallCommand = oldRecall
	})

	app.registerSkillsTools("ws-1")
	app.registerRecallTools("ws-1")
	want := []string{"mcp_servers.gotack-skills", "mcp_servers.gotack-recall"}
	if !reflect.DeepEqual(fake.removedKeys, want) {
		t.Fatalf("removed keys = %#v, want %#v", fake.removedKeys, want)
	}
}

package skillmanage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateInCategory(t *testing.T) {
	manager := newTestManager(t)
	content := skillText("deploy-check", "Use when checking a deployment.", "Run the health check.")
	result := mustApply(t, manager, Operation{
		Action: actionCreate, Name: "deploy-check", Category: "devops", Content: content,
	})
	if result.Results[0].Path != "devops/deploy-check" {
		t.Fatalf("created path = %q", result.Results[0].Path)
	}
	if got := readFile(t, filepath.Join(manager.Root(), "devops", "deploy-check", "SKILL.md")); got != content {
		t.Fatalf("created content = %q", got)
	}
}

func TestCreateValidation(t *testing.T) {
	manager := newTestManager(t)
	tests := []struct {
		name      string
		operation Operation
		contains  string
	}{
		{"uppercase name", Operation{Action: actionCreate, Name: "Bad", Content: skillText("Bad", "Use when testing names.", "Run it.")}, "invalid skill name"},
		{"underscore name", Operation{Action: actionCreate, Name: "bad_name", Content: skillText("bad_name", "Use when testing names.", "Run it.")}, "invalid skill name"},
		{"name mismatch", Operation{Action: actionCreate, Name: "one-name", Content: skillText("other-name", "Use when testing names.", "Run it.")}, "must match"},
		{"long description", Operation{Action: actionCreate, Name: "long-description", Content: skillText("long-description", strings.Repeat("x", MaxDescriptionChars+1), "Run it.")}, "limit: 60"},
		{"missing body", Operation{Action: actionCreate, Name: "missing-body", Content: "---\nname: missing-body\ndescription: Use when testing bodies.\n---\n"}, "needs instructions"},
		{"invalid category", Operation{Action: actionCreate, Name: "good-name", Category: "bad_category", Content: skillText("good-name", "Use when testing categories.", "Run it.")}, "invalid category"},
		{"oversized", Operation{Action: actionCreate, Name: "too-large", Content: skillText("too-large", "Use when testing limits.", strings.Repeat("x", MaxSkillContent))}, "limit: 100000"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := manager.Apply(context.Background(), []Operation{test.operation})
			if result.Success || !strings.Contains(result.Error, test.contains) {
				t.Fatalf("result = %+v, want %q", result, test.contains)
			}
		})
	}
}

func TestCategoryCannotHideSkillInsideAnotherSkill(t *testing.T) {
	manager := newTestManager(t)
	mustApply(t, manager, Operation{Action: actionCreate, Name: "parent", Content: skillText("parent", "Use when testing a parent.", "Run it.")})
	result := manager.Apply(context.Background(), []Operation{{
		Action: actionCreate, Name: "child", Category: "parent", Content: skillText("child", "Use when testing a child.", "Run it."),
	}})
	if result.Success || !strings.Contains(result.Error, "collides") {
		t.Fatalf("nested create = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(manager.Root(), "parent", "child")); !os.IsNotExist(err) {
		t.Fatalf("child path exists: %v", err)
	}
}

func TestSupportFileLifecycleAndPathValidation(t *testing.T) {
	manager := newTestManager(t)
	mustApply(t, manager, Operation{Action: actionCreate, Name: "with-files", Content: skillText("with-files", "Use when testing support files.", "Run it.")})
	for _, directory := range []string{"references", "templates", "scripts", "assets"} {
		content := "content for " + directory
		path := directory + "/item.txt"
		mustApply(t, manager, Operation{Action: actionWriteFile, Name: "with-files", FilePath: path, FileContent: &content})
		if got := readFile(t, filepath.Join(manager.Root(), "with-files", filepath.FromSlash(path))); got != content {
			t.Fatalf("support content %s = %q", path, got)
		}
		mustApply(t, manager, Operation{Action: actionRemoveFile, Name: "with-files", FilePath: path})
		if _, err := os.Stat(filepath.Join(manager.Root(), "with-files", filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("removed %s still exists: %v", path, err)
		}
	}

	content := "unsafe"
	for _, path := range []string{"../escape", "references/../escape", "C:/escape", "/escape", "examples/nope"} {
		result := manager.Apply(context.Background(), []Operation{{
			Action: actionWriteFile, Name: "with-files", FilePath: path, FileContent: &content,
		}})
		if result.Success {
			t.Fatalf("unsafe path %q accepted", path)
		}
	}
	tooLarge := strings.Repeat("x", MaxSupportFileBytes+1)
	result := manager.Apply(context.Background(), []Operation{{
		Action: actionWriteFile, Name: "with-files", FilePath: "assets/large.bin", FileContent: &tooLarge,
	}})
	if result.Success || !strings.Contains(result.Error, "1 MiB") {
		t.Fatalf("large support result = %+v", result)
	}
}

func TestBatchRollbackRestoresExistingSkill(t *testing.T) {
	manager := newTestManager(t)
	original := skillText("rollback", "Use when testing rollback.", "Original procedure.")
	mustApply(t, manager, Operation{Action: actionCreate, Name: "rollback", Content: original})
	one := "First procedure."
	two := "Second procedure."
	result := manager.Apply(context.Background(), []Operation{
		{Action: actionPatch, Name: "rollback", OldString: "Original procedure.", NewString: &one},
		{Action: actionPatch, Name: "rollback", OldString: "missing text", NewString: &two},
	})
	if result.Success || result.FailedIndex == nil || *result.FailedIndex != 1 {
		t.Fatalf("batch = %+v", result)
	}
	if got := readFile(t, filepath.Join(manager.Root(), "rollback", "SKILL.md")); got != original {
		t.Fatalf("rollback content changed:\n%s", got)
	}
}

func TestPatchRequiresUniqueExactText(t *testing.T) {
	manager := newTestManager(t)
	original := skillText("exact-patch", "Use when testing exact patches.", "repeat marker\nrepeat marker")
	mustApply(t, manager, Operation{Action: actionCreate, Name: "exact-patch", Content: original})
	replacement := "updated"
	for _, old := range []string{"repeat  marker", "repeat marker"} {
		result := manager.Apply(context.Background(), []Operation{{Action: actionPatch, Name: "exact-patch", OldString: old, NewString: &replacement}})
		if result.Success {
			t.Fatalf("non-exact or ambiguous patch %q succeeded", old)
		}
	}
	if got := readFile(t, filepath.Join(manager.Root(), "exact-patch", "SKILL.md")); got != original {
		t.Fatalf("failed patch changed content:\n%s", got)
	}
}

func TestBatchRollbackRemovesNewSkill(t *testing.T) {
	manager := newTestManager(t)
	replacement := "new"
	result := manager.Apply(context.Background(), []Operation{
		{Action: actionCreate, Name: "new-skill", Content: skillText("new-skill", "Use when testing new rollback.", "Procedure.")},
		{Action: actionPatch, Name: "new-skill", OldString: "missing", NewString: &replacement},
	})
	if result.Success {
		t.Fatalf("batch unexpectedly succeeded: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(manager.Root(), "new-skill")); !os.IsNotExist(err) {
		t.Fatalf("new skill survived rollback: %v", err)
	}
}

func TestBatchRollbackSpansEveryTouchedSkill(t *testing.T) {
	manager := newTestManager(t)
	for _, name := range []string{"first-skill", "second-skill"} {
		mustApply(t, manager, Operation{Action: actionCreate, Name: name, Content: skillText(name, "Use when testing multi-skill rollback.", "Original.")})
	}
	changed := "Changed."
	result := manager.Apply(context.Background(), []Operation{
		{Action: actionPatch, Name: "first-skill", OldString: "Original.", NewString: &changed},
		{Action: actionPatch, Name: "second-skill", OldString: "Original.", NewString: &changed},
		{Action: actionPatch, Name: "first-skill", OldString: "not present", NewString: &changed},
	})
	if result.Success {
		t.Fatal("batch unexpectedly succeeded")
	}
	for _, name := range []string{"first-skill", "second-skill"} {
		if got := readFile(t, filepath.Join(manager.Root(), name, "SKILL.md")); !strings.Contains(got, "Original.") || strings.Contains(got, "Changed.") {
			t.Fatalf("%s was not restored: %s", name, got)
		}
	}
}

func TestBatchLimitsAndClobberGuard(t *testing.T) {
	manager := newTestManager(t)
	operations := make([]Operation, MaxOperations+1)
	for index := range operations {
		name := fmt.Sprintf("skill-%d", index)
		operations[index] = Operation{Action: actionCreate, Name: name, Content: skillText(name, "Use when testing batch limits.", "Run it.")}
	}
	if result := manager.Apply(context.Background(), operations); result.Success || !strings.Contains(result.Error, "capped") {
		t.Fatalf("large batch = %+v", result)
	}

	mustApply(t, manager, Operation{Action: actionCreate, Name: "clobber", Content: skillText("clobber", "Use when testing clobbering.", "Run it.")})
	original := "old"
	mustApply(t, manager, Operation{Action: actionWriteFile, Name: "clobber", FilePath: "references/file.txt", FileContent: &original})
	patched := "patched"
	overwrite := "overwrite"
	result := manager.Apply(context.Background(), []Operation{
		{Action: actionPatch, Name: "clobber", FilePath: "references/file.txt", OldString: "old", NewString: &patched},
		{Action: actionWriteFile, Name: "clobber", FilePath: "references/file.txt", FileContent: &overwrite},
	})
	if result.Success || !strings.Contains(result.Error, "discard") {
		t.Fatalf("clobber batch = %+v", result)
	}
	if got := readFile(t, filepath.Join(manager.Root(), "clobber", "references", "file.txt")); got != original {
		t.Fatalf("preflight changed file to %q", got)
	}
}

func TestDeleteMustBeSoleAndRemainsInsideRoot(t *testing.T) {
	manager := newTestManager(t)
	mustApply(t, manager, Operation{Action: actionCreate, Name: "delete-me", Content: skillText("delete-me", "Use when testing deletion.", "Run it.")})
	result := manager.Apply(context.Background(), []Operation{
		{Action: actionDelete, Name: "delete-me"},
		{Action: actionCreate, Name: "other", Content: skillText("other", "Use when testing deletion.", "Run it.")},
	})
	if result.Success || !strings.Contains(result.Error, "sole") {
		t.Fatalf("mixed delete = %+v", result)
	}
	mustApply(t, manager, Operation{Action: actionDelete, Name: "delete-me"})
	if _, err := os.Stat(manager.Root()); err != nil {
		t.Fatalf("skill root removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(manager.Root(), "delete-me")); !os.IsNotExist(err) {
		t.Fatalf("skill remained: %v", err)
	}
}

func TestBackgroundReviewOwnershipAndFreshRead(t *testing.T) {
	manager := newTestManager(t)
	foreground := skillText("user-owned", "Use when testing user ownership.", "Original.")
	mustApply(t, manager, Operation{Action: actionCreate, Name: "user-owned", Content: foreground})
	review := RequestMeta{SessionID: "review-a", BackgroundReview: true}
	mustView(t, manager, "user-owned", "", review)
	replacement := "Changed."
	blocked := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionPatch, Name: "user-owned", OldString: "Original.", NewString: &replacement}}, review)
	if blocked.Success || !strings.Contains(blocked.Error, "not agent-owned") {
		t.Fatalf("user-owned patch = %+v", blocked)
	}

	created := manager.ApplyWithMeta(context.Background(), []Operation{{
		Action: actionCreate, Name: "agent-owned", Content: skillText("agent-owned", "Use when testing agent ownership.", "Original."),
	}}, review)
	if !created.Success {
		t.Fatalf("background create = %+v", created)
	}
	blocked = manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionPatch, Name: "agent-owned", OldString: "Original.", NewString: &replacement}}, review)
	if blocked.Success || !strings.Contains(blocked.Error, "Crush view") {
		t.Fatalf("patch without fresh view = %+v", blocked)
	}
	mustView(t, manager, "agent-owned", "", review)
	otherReview := RequestMeta{SessionID: "review-b", BackgroundReview: true}
	blocked = manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionPatch, Name: "agent-owned", OldString: "Original.", NewString: &replacement}}, otherReview)
	if blocked.Success {
		t.Fatalf("cross-session read mark leaked: %+v", blocked)
	}
	mustView(t, manager, "agent-owned", "", review)
	if patched := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionPatch, Name: "agent-owned", OldString: "Original.", NewString: &replacement}}, review); !patched.Success {
		t.Fatalf("fresh patch = %+v", patched)
	}
	second := "Again."
	blocked = manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionPatch, Name: "agent-owned", OldString: replacement, NewString: &second}}, review)
	if blocked.Success || !strings.Contains(blocked.Error, "Crush view") {
		t.Fatalf("consumed read mark reused: %+v", blocked)
	}
	if len(manager.readMarks[review.SessionID]) != 0 {
		t.Fatalf("read marks were not consumed: %v", manager.readMarks[review.SessionID])
	}
}

func TestFailedBackgroundCreateDoesNotPersistOwnership(t *testing.T) {
	manager := newTestManager(t)
	review := RequestMeta{SessionID: "review", BackgroundReview: true}
	replacement := "replacement"
	result := manager.ApplyWithMeta(context.Background(), []Operation{
		{Action: actionCreate, Name: "rolled-back", Content: skillText("rolled-back", "Use when testing ownership rollback.", "Original.")},
		{Action: actionPatch, Name: "rolled-back", OldString: "missing", NewString: &replacement},
	}, review)
	if result.Success {
		t.Fatal("batch unexpectedly succeeded")
	}
	owned, err := manager.loadOwnership()
	if err != nil {
		t.Fatal(err)
	}
	if owned["rolled-back"] {
		t.Fatal("rolled-back skill persisted as agent-owned")
	}
}

func TestLegacyOwnershipManifestMigratesOnNextOwnershipWrite(t *testing.T) {
	manager := newTestManager(t)
	mustApply(t, manager, Operation{
		Action: actionCreate, Name: "legacy-owned",
		Content: skillText("legacy-owned", "Use when testing legacy ownership.", "Original."),
	})
	legacy := "{\"version\":1,\"agent_owned\":[\"legacy-owned\"]}\n"
	if err := os.WriteFile(filepath.Join(manager.Root(), legacyOwnershipFileName), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	review := RequestMeta{SessionID: "review-migrate", BackgroundReview: true}
	mustView(t, manager, "legacy-owned", "", review)
	replacement := "Updated."
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{
		Action: actionPatch, Name: "legacy-owned", OldString: "Original.", NewString: &replacement,
	}}, review); !result.Success {
		t.Fatalf("legacy ownership was not honored: %+v", result)
	}
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{
		Action: actionCreate, Name: "new-owned",
		Content: skillText("new-owned", "Use when testing ownership migration.", "Run it."),
	}}, review); !result.Success {
		t.Fatalf("ownership migration write failed: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(manager.Root(), ownershipFileName)); err != nil {
		t.Fatalf("new ownership manifest missing: %v", err)
	}
	owned, err := manager.loadOwnership()
	if err != nil {
		t.Fatal(err)
	}
	if !owned["legacy-owned"] || !owned["new-owned"] {
		t.Fatalf("migrated ownership = %v", owned)
	}
}

func TestUnreadableOwnershipFailsClosed(t *testing.T) {
	manager := newTestManager(t)
	if err := os.WriteFile(filepath.Join(manager.Root(), ownershipFileName), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	review := RequestMeta{SessionID: "review", BackgroundReview: true}
	result := manager.ApplyWithMeta(context.Background(), []Operation{{
		Action: actionCreate, Name: "must-not-create", Content: skillText("must-not-create", "Use when testing failed ownership.", "Run it."),
	}}, review)
	if result.Success || !strings.Contains(result.Error, "ownership") {
		t.Fatalf("corrupt manifest result = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(manager.Root(), "must-not-create")); !os.IsNotExist(err) {
		t.Fatalf("skill created despite corrupt manifest: %v", err)
	}
}

func TestBackgroundReadMarksAreBounded(t *testing.T) {
	manager := newTestManager(t)
	created := manager.ApplyWithMeta(context.Background(), []Operation{{
		Action: actionCreate, Name: "bounded", Content: skillText("bounded", "Use when testing bounded reads.", "Run it."),
	}}, RequestMeta{SessionID: "creator", BackgroundReview: true})
	if !created.Success {
		t.Fatal(created.Error)
	}
	for index := 0; index <= maxReadSessions; index++ {
		sessionID := fmt.Sprintf("review-%d", index)
		mustView(t, manager, "bounded", "", RequestMeta{SessionID: sessionID, BackgroundReview: true})
	}
	if len(manager.readMarks) != maxReadSessions {
		t.Fatalf("read-mark session count = %d, want %d", len(manager.readMarks), maxReadSessions)
	}
}

func TestBackgroundSupportFileReadGuard(t *testing.T) {
	manager := newTestManager(t)
	review := RequestMeta{SessionID: "review", BackgroundReview: true}
	created := manager.ApplyWithMeta(context.Background(), []Operation{{
		Action: actionCreate, Name: "support-guard", Content: skillText("support-guard", "Use when testing support guards.", "Original."),
	}}, review)
	if !created.Success {
		t.Fatal(created.Error)
	}
	original := "support original"
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionWriteFile, Name: "support-guard", FilePath: "references/info.md", FileContent: &original}}, review); !result.Success {
		t.Fatalf("new support file = %+v", result)
	}
	updated := "support updated"
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionWriteFile, Name: "support-guard", FilePath: "references/info.md", FileContent: &updated}}, review); result.Success {
		t.Fatal("overwrite without view succeeded")
	}
	mustView(t, manager, "support-guard", "", review)
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionWriteFile, Name: "support-guard", FilePath: "references/info.md", FileContent: &updated}}, review); result.Success {
		t.Fatal("SKILL.md view authorized support-file overwrite")
	}
	mustView(t, manager, "support-guard", "references/info.md", review)
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionWriteFile, Name: "support-guard", FilePath: "references/info.md", FileContent: &updated}}, review); !result.Success {
		t.Fatalf("viewed support overwrite = %+v", result)
	}
}

func TestViewReturnsExactFileAndMarksOnlyThatTarget(t *testing.T) {
	manager := newTestManager(t)
	review := RequestMeta{SessionID: "review-view", BackgroundReview: true}
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionCreate, Name: "viewable", Content: skillText("viewable", "Use when testing views.", "Original.")}}, review); !result.Success {
		t.Fatalf("background create = %+v", result)
	}
	support := "reference text"
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionWriteFile, Name: "viewable", FilePath: "references/info.md", FileContent: &support}}, review); !result.Success {
		t.Fatalf("background support create = %+v", result)
	}

	result := mustView(t, manager, "viewable", "references/info.md", review)
	if result.FilePath != "references/info.md" || result.Content != support || !result.Success {
		t.Fatalf("view result = %+v", result)
	}
	updated := "updated reference"
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionWriteFile, Name: "viewable", FilePath: "references/info.md", FileContent: &updated}}, review); !result.Success {
		t.Fatalf("viewed support write = %+v", result)
	}
	// A mark for one file must never authorize a different existing file.
	replacement := "Changed."
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionPatch, Name: "viewable", OldString: "Original.", NewString: &replacement}}, review); result.Success {
		t.Fatal("support-file view authorized an unrelated SKILL.md patch")
	}
}

func TestBackgroundDeleteRequiresFreshSkillView(t *testing.T) {
	manager := newTestManager(t)
	review := RequestMeta{SessionID: "review-delete", BackgroundReview: true}
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionCreate, Name: "delete-review", Content: skillText("delete-review", "Use when testing guarded deletion.", "Keep.")}}, review); !result.Success {
		t.Fatalf("background create = %+v", result)
	}
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionCreate, Name: "umbrella", Content: skillText("umbrella", "Use when testing an umbrella.", "Absorb procedures.")}}, review); !result.Success {
		t.Fatalf("background umbrella create = %+v", result)
	}
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionDelete, Name: "delete-review"}}, review); result.Success || !strings.Contains(result.Error, "skill_view") {
		t.Fatalf("delete without view = %+v", result)
	}
	mustView(t, manager, "delete-review", "", review)
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionDelete, Name: "delete-review"}}, review); result.Success || !strings.Contains(result.Error, "absorbed_into") {
		t.Fatalf("delete without consolidation target = %+v", result)
	}
	mustView(t, manager, "delete-review", "", review)
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionDelete, Name: "delete-review", AbsorbedInto: "delete-review"}}, review); result.Success || !strings.Contains(result.Error, "cannot equal") {
		t.Fatalf("self consolidation target = %+v", result)
	}
	mustView(t, manager, "delete-review", "", review)
	if result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionDelete, Name: "delete-review", AbsorbedInto: "missing-umbrella"}}, review); result.Success || !strings.Contains(result.Error, "not agent-owned") {
		t.Fatalf("unowned consolidation target = %+v", result)
	}
	mustView(t, manager, "delete-review", "", review)
	result := manager.ApplyWithMeta(context.Background(), []Operation{{Action: actionDelete, Name: "delete-review", AbsorbedInto: "umbrella"}}, review)
	if !result.Success {
		t.Fatalf("viewed consolidated delete = %+v", result)
	}
	if result.Results[0].Path != ".archive/delete-review" {
		t.Fatalf("archive path = %q", result.Results[0].Path)
	}
	if _, err := os.Stat(filepath.Join(manager.Root(), ".archive", "delete-review", "SKILL.md")); err != nil {
		t.Fatalf("archived skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(manager.Root(), "delete-review")); !os.IsNotExist(err) {
		t.Fatalf("active skill still exists: %v", err)
	}
}

func TestSymlinkEscapeIsRejected(t *testing.T) {
	manager := newTestManager(t)
	mustApply(t, manager, Operation{Action: actionCreate, Name: "links", Content: skillText("links", "Use when testing links.", "Run it.")})
	external := t.TempDir()
	link := filepath.Join(manager.Root(), "links", "references")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	content := "must stay inside"
	result := manager.Apply(context.Background(), []Operation{{Action: actionWriteFile, Name: "links", FilePath: "references/escape.txt", FileContent: &content}})
	if result.Success || !strings.Contains(strings.ToLower(result.Error), "symlink") {
		t.Fatalf("symlink write = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(external, "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("external target changed: %v", err)
	}
}

func TestContextCancellationStopsBeforeMutation(t *testing.T) {
	manager := newTestManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := manager.Apply(ctx, []Operation{{Action: actionCreate, Name: "cancelled", Content: skillText("cancelled", "Use when testing cancellation.", "Run it.")}})
	if result.Success || !strings.Contains(result.Error, "canceled") {
		t.Fatalf("cancelled result = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(manager.Root(), "cancelled")); !os.IsNotExist(err) {
		t.Fatalf("cancelled skill exists: %v", err)
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	manager, err := New(filepath.Join(t.TempDir(), "skills"))
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func skillText(name, description, body string) string {
	return fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n# %s\n\n%s\n", name, description, name, body)
}

func mustApply(t *testing.T, manager *Manager, operations ...Operation) Result {
	t.Helper()
	result := manager.Apply(context.Background(), operations)
	if !result.Success {
		t.Fatalf("Apply: %+v", result)
	}
	return result
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func mustView(t *testing.T, manager *Manager, name, filePath string, meta RequestMeta) ViewResult {
	t.Helper()
	result, err := manager.View(context.Background(), name, filePath, meta)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

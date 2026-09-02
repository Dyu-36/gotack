package guard

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

func TestMarkAndContainsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", UnattendedRosterFileName)
	if RosterContains(path, "sess-zalo") {
		t.Fatal("missing roster must contain nothing")
	}
	if err := MarkUnattendedSession(path, "sess-zalo"); err != nil {
		t.Fatalf("mark: %v", err)
	}
	if !RosterContains(path, "sess-zalo") {
		t.Fatal("marked session must be reported unattended")
	}
	if RosterContains(path, "sess-ui") {
		t.Fatal("unmarked session must not be unattended")
	}
}

func TestMarkIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), UnattendedRosterFileName)
	for i := 0; i < 3; i++ {
		if err := MarkUnattendedSession(path, "sess-1"); err != nil {
			t.Fatalf("mark %d: %v", i, err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range splitLines(string(data)) {
		if line == `    "sess-1",` || line == `    "sess-1"` {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("duplicate marks must keep one entry, file:\n%s", data)
	}
}

func TestContainsFailsOpenOnUnreadableRoster(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"malformed": "{not-json",
		"empty":     "",
		"wrongtype": `{"sessions": 42}`,
	}
	for name, content := range cases {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if RosterContains(path, "sess-1") {
			t.Fatalf("%s roster must fail open to interactive", name)
		}
	}
}

func TestMarkRequiresSessionID(t *testing.T) {
	if err := MarkUnattendedSession(filepath.Join(t.TempDir(), UnattendedRosterFileName), ""); err == nil {
		t.Fatal("marking an empty session id must fail")
	}
	if RosterContains("ignored", "") {
		t.Fatal("empty session id is never unattended")
	}
}

func TestMarkCapsRosterSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), UnattendedRosterFileName)
	for i := 0; i < rosterCap+10; i++ {
		if err := MarkUnattendedSession(path, "sess-"+strconv.Itoa(i)); err != nil {
			t.Fatalf("mark %d: %v", i, err)
		}
	}

	if !RosterContains(path, "sess-"+strconv.Itoa(rosterCap+9)) {
		t.Fatal("newest session must survive the cap trim")
	}
	if RosterContains(path, "sess-0") {
		t.Fatal("oldest session must be trimmed past the cap")
	}
}

func TestReviewRosterLifecycleIsDistinct(t *testing.T) {
	dir := t.TempDir()
	reviewPath := filepath.Join(dir, ReviewRosterFileName)
	unattendedPath := filepath.Join(dir, UnattendedRosterFileName)

	if err := MarkReviewSession(reviewPath, "review-1"); err != nil {
		t.Fatalf("mark review: %v", err)
	}
	if !ReviewRosterContains(reviewPath, "review-1") {
		t.Fatal("marked review session must be present")
	}
	if RosterContains(unattendedPath, "review-1") {
		t.Fatal("review roster must not implicitly mark the unattended roster")
	}
	if err := UnmarkReviewSession(reviewPath, "review-1"); err != nil {
		t.Fatalf("unmark review: %v", err)
	}
	if ReviewRosterContains(reviewPath, "review-1") {
		t.Fatal("unmarked review session must be absent")
	}
	if err := UnmarkReviewSession(reviewPath, "review-1"); err != nil {
		t.Fatalf("repeated unmark must be idempotent: %v", err)
	}
}

func TestReviewRosterRequiresSessionID(t *testing.T) {
	path := filepath.Join(t.TempDir(), ReviewRosterFileName)
	if err := MarkReviewSession(path, ""); err == nil {
		t.Fatal("marking an empty review session id must fail")
	}
	if err := UnmarkReviewSession(path, ""); err == nil {
		t.Fatal("unmarking an empty review session id must fail")
	}
}

func TestConcurrentReviewCleanupKeepsNewMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), ReviewRosterFileName)
	if err := MarkReviewSession(path, "old"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := UnmarkReviewSession(path, "old"); err != nil {
			t.Errorf("unmark old: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := MarkReviewSession(path, "new"); err != nil {
			t.Errorf("mark new: %v", err)
		}
	}()
	wg.Wait()
	if !ReviewRosterContains(path, "new") {
		t.Fatal("concurrent cleanup lost the new review marker")
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			out = append(out, line)
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

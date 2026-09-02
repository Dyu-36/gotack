package recall

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"testing"
)

func ftsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	createFixture(t, dir, func(db *sql.DB) {
		rows := []struct {
			id   string
			text string
		}{
			{"m-1", "deploy the kubernetes cluster tonight"},
			{"m-2", "rollback the kubernetes cluster after the outage"},
			{"m-3", "deployment pipeline is green"},
			{"m-4", "lunch order for the team"},
			{"m-5", "cluster then kubernetes in reversed order"},
		}
		for i, row := range rows {
			at := int64(1_700_000_000_000 + i)
			sessionID := "s-" + row.id
			seedSession(t, db, sessionID, row.id, at)
			seedMessage(t, db, row.id, sessionID, "user",
				partsArray(partFixture(t, "text", map[string]string{"text": row.text})),
				at, at)
		}
	})
	return dir
}

func hitIDs(results []DiscoveryResult) string {
	ids := make([]string, 0, len(results))
	for _, result := range results {
		ids = append(ids, result.MatchMessageID)
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

func TestSearchFtsSyntax(t *testing.T) {
	store := newTestStore(t, ftsFixture(t))
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"two words are AND", "kubernetes cluster", "m-1,m-2,m-5"},
		{"quoted run is an exact phrase", `"kubernetes cluster"`, "m-1,m-2"},
		{"OR unions both sides", "deploy OR lunch", "m-1,m-4"},
		{"NOT subtracts the right side", "kubernetes NOT rollback", "m-1,m-5"},
		{"trailing star matches by prefix", "deploy*", "m-1,m-3"},
		{"lower-case and is a word, not an operator", "and", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := store.Search(context.Background(), tc.query, 10)
			if err != nil {
				t.Fatalf("Search(%q): %v", tc.query, err)
			}
			if got := hitIDs(results); got != tc.want {
				t.Fatalf("Search(%q) matched %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

func TestBuildMatchRendersSafeExpressions(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"single term", "kubernetes", `"kubernetes"`},
		{"implicit AND", "staging credentials", `"staging" "credentials"`},
		{"explicit operators pass through", "lunch AND kubernetes", `"lunch" AND "kubernetes"`},
		{"or between phrases", `deploy OR "roll back"`, `"deploy" OR "roll back"`},
		{"prefix star survives quoting", "deploy*", `"deploy"*`},
		{"quoted phrase can take a prefix", `"exact phrase"*`, `"exact phrase"*`},
		{"embedded quote is doubled", `say "hi`, `"say" "hi"`},
		{"column filter is neutralised", "title:secret", `"title:secret"`},
		{"near is neutralised", "NEAR/3", `"NEAR/3"`},
		{"dangling operator falls back to words", "NOT kubernetes", `"kubernetes"`},
		{"operators alone are literal words", "AND OR NOT", `"AND" "OR" "NOT"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildMatch(tc.query)
			if err != nil {
				t.Fatalf("buildMatch(%q): %v", tc.query, err)
			}
			if got != tc.want {
				t.Fatalf("buildMatch(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}

	for _, query := range []string{"", "   ", `"`, "...", "-", "***"} {
		if _, err := buildMatch(query); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("buildMatch(%q) err = %v, want ErrInvalidQuery", query, err)
		}
	}
}

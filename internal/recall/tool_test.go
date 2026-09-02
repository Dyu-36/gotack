package recall

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
)

func callTool(t *testing.T, store *Store, args string) map[string]any {
	t.Helper()
	text, err := Tool(store).Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("session_search(%s): %v", args, err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("decode response: %v\n%s", err, text)
	}
	return payload
}

func TestToolImplementsFourHermesShapes(t *testing.T) {
	store := newTestStore(t, standardFixture(t, t.TempDir()))

	browse := callTool(t, store, `{}`)
	if browse["mode"] != "browse" || browse["count"].(float64) != 2 {
		t.Fatalf("browse = %+v", browse)
	}

	discover := callTool(t, store, `{"query":"kubernetes"}`)
	if discover["mode"] != "discover" || discover["detail"] != "adaptive" || discover["count"].(float64) != 2 {
		t.Fatalf("discover = %+v", discover)
	}
	results := discover["results"].([]any)
	if results[0].(map[string]any)["detail"] != "full" || results[1].(map[string]any)["detail"] != "adaptive" {
		t.Fatalf("adaptive hydration = %+v", results)
	}
	if got := callTool(t, store, `{"query":"healthy"}`)["count"].(float64); got != 0 {
		t.Fatalf("default roles exposed tool output: count=%v", got)
	}
	if got := callTool(t, store, `{"query":"healthy","role_filter":"tool"}`)["count"].(float64); got != 1 {
		t.Fatalf("explicit tool role count=%v", got)
	}
	if got := callTool(t, store, `{"query":"kubernetes","current_session_id":"sess-deploy"}`)["count"].(float64); got != 1 {
		t.Fatalf("current session was not excluded: count=%v", got)
	}

	read := callTool(t, store, `{"query":"ignored","session_id":"sess-deploy"}`)
	if read["mode"] != "read" || read["message_count"].(float64) != 3 {
		t.Fatalf("read precedence = %+v", read)
	}

	scroll := callTool(t, store, `{"query":"ignored","session_id":"sess-deploy","around_message_id":"deploy-2","window":1}`)
	if scroll["mode"] != "scroll" || scroll["window"].(float64) != 1 {
		t.Fatalf("scroll precedence = %+v", scroll)
	}
	messages := scroll["messages"].([]any)
	if len(messages) != 3 || messages[1].(map[string]any)["anchor"] != true {
		t.Fatalf("scroll window = %+v", messages)
	}
}

func TestToolExcludesCurrentSessionLineageFromDiscoveryAndBrowse(t *testing.T) {
	dir := createFixture(t, t.TempDir(), func(db *sql.DB) {
		seedSession(t, db, "root", "Root", 100)
		seedSession(t, db, "current", "Current", 300)
		seedSession(t, db, "child", "Child", 400)
		if _, err := db.Exec("UPDATE sessions SET parent_session_id = ? WHERE id = ?", "root", "current"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("UPDATE sessions SET parent_session_id = ? WHERE id = ?", "current", "child"); err != nil {
			t.Fatal(err)
		}
		seedMessage(t, db, "root-m", "root", "user", textParts(t, "lineage needle root"), 1, 1)
		seedMessage(t, db, "current-m", "current", "user", textParts(t, "lineage needle current"), 2, 2)
		seedMessage(t, db, "child-m", "child", "assistant", textParts(t, "lineage needle child"), 3, 3)
	})
	store := newTestStore(t, dir)
	query := callTool(t, store, `{"query":"lineage","current_session_id":"current","limit":10}`)
	if query["count"].(float64) != 0 {
		t.Fatalf("lineage sessions leaked from discovery: %+v", query)
	}
	browse := callTool(t, store, `{"current_session_id":"current","limit":10}`)
	if browse["count"].(float64) != 0 {
		t.Fatalf("lineage sessions leaked from browse: %+v", browse)
	}
}

func TestToolResponseContentCaps(t *testing.T) {
	dir := createFixture(t, t.TempDir(), func(db *sql.DB) {
		seedSession(t, db, "large", "Large", 100)
		for i := 0; i < 25; i++ {
			content := "needle " + strings.Repeat("ệ", 3000)
			seedMessage(t, db, fmtID(i), "large", "user", textParts(t, content), int64(i), int64(i))
		}
	})
	payload := callTool(t, newTestStore(t, dir), `{"query":"needle","detail":"full"}`)
	total, largest, truncated := contentStats(payload)
	if total > maxResponseContentBytes {
		t.Fatalf("content total=%d, cap=%d", total, maxResponseContentBytes)
	}
	if largest > maxMessageContentBytes {
		t.Fatalf("largest content=%d, cap=%d", largest, maxMessageContentBytes)
	}
	if !truncated {
		t.Fatal("long content did not report truncation")
	}
	result := payload["results"].([]any)[0].(map[string]any)
	for _, key := range []string{"bookend_start", "bookend_end"} {
		for _, raw := range result[key].([]any) {
			if got := len(raw.(map[string]any)["content"].(string)); got > maxBookendContentBytes {
				t.Fatalf("%s content=%d, cap=%d", key, got, maxBookendContentBytes)
			}
		}
	}
}

func TestToolSchemaUsesStringMessageIDs(t *testing.T) {
	tool := Tool(newTestStore(t, standardFixture(t, t.TempDir())))
	var schema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(tool.Schema, &schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	if schema.Properties["around_message_id"].Type != "string" {
		t.Fatalf("around_message_id type = %q", schema.Properties["around_message_id"].Type)
	}
	if _, err := tool.Handler(context.Background(), nil); err == nil {
		t.Fatal("absent argument object must fail; {} is browse")
	}
	zero, huge := 0, 999
	if requestLimit(&zero) != 1 || requestLimit(&huge) != 10 ||
		requestWindow(&zero) != 1 || requestWindow(&huge) != 20 {
		t.Fatal("explicit limits must use Hermes clamps")
	}
}

func contentStats(value any) (total, largest int, truncated bool) {
	switch value := value.(type) {
	case map[string]any:
		if content, ok := value["content"].(string); ok {
			size := len(content)
			total += size
			if size > largest {
				largest = size
			}
		}
		if value["content_truncated"] == true {
			truncated = true
		}
		for _, child := range value {
			childTotal, childLargest, childTruncated := contentStats(child)
			total += childTotal
			if childLargest > largest {
				largest = childLargest
			}
			truncated = truncated || childTruncated
		}
	case []any:
		for _, child := range value {
			childTotal, childLargest, childTruncated := contentStats(child)
			total += childTotal
			if childLargest > largest {
				largest = childLargest
			}
			truncated = truncated || childTruncated
		}
	}
	return total, largest, truncated
}

package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileLoggerAppendsJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "audit.jsonl")
	l := NewFileLogger(path)

	l.Log("update_task", "ok", map[string]any{"taskId": "abc"})
	l.Log("bulk_move_tasks", "read-only-blocked", map[string]any{"count": 3})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var e Entry
	if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
		t.Fatalf("line 1 not valid json: %v", err)
	}
	if e.Tool != "update_task" || e.Outcome != "ok" || e.TS == "" {
		t.Errorf("unexpected entry: %+v", e)
	}
	if e.Args["taskId"] != "abc" {
		t.Errorf("args not persisted: %+v", e.Args)
	}
	var e2 Entry
	if err := json.Unmarshal([]byte(lines[1]), &e2); err != nil {
		t.Fatalf("line 2 not valid json: %v", err)
	}
	if e2.Outcome != "read-only-blocked" {
		t.Errorf("unexpected outcome: %s", e2.Outcome)
	}
}

func TestNoopLogger(t *testing.T) {
	l := NewNoopLogger()
	l.Log("x", "ok", nil) // не паникует
}

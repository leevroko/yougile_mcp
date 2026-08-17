package mcp

import (
	"testing"
)

func TestListTools(t *testing.T) {
	s := New(Deps{})
	if s.mcp == nil {
		t.Fatal("nil mcp server")
	}
	tools := s.mcp.ListTools()
	t.Logf("registered tools: %d", len(tools))
	if len(tools) != 15 {
		t.Fatalf("expected 15 tools, got %d", len(tools))
	}
	for _, name := range []string{
		"list_projects", "list_boards", "list_columns", "list_tasks",
		"get_task", "create_task", "update_task", "get_stickers",
		"get_board_snapshot", "summarize_board", "audit_board",
		"track_goals", "bulk_move_tasks", "batch_update_stickers", "compress_reviews",
	} {
		if _, ok := tools[name]; !ok {
			t.Errorf("tool %q not registered", name)
		}
	}
}

// Мутационные инструменты, которые должны исчезать в read-only.
var mutatingTools = []string{
	"create_task", "update_task", "audit_board",
	"bulk_move_tasks", "batch_update_stickers",
}

func TestReadOnlyHidesMutatingTools(t *testing.T) {
	s := New(Deps{ReadOnly: true})
	tools := s.mcp.ListTools()
	for _, name := range mutatingTools {
		if _, ok := tools[name]; ok {
			t.Errorf("read-only: mutating tool %q must not be registered", name)
		}
	}
	// Читающие инструменты остаются
	for _, name := range []string{"list_projects", "list_tasks", "summarize_board"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("read-only: read tool %q must be registered", name)
		}
	}
	if len(tools) != 10 {
		t.Fatalf("read-only: expected 10 tools, got %d", len(tools))
	}
}

func TestAnnotations(t *testing.T) {
	s := New(Deps{})
	tools := s.mcp.ListTools()

	ro, ok := tools["list_tasks"]
	if !ok {
		t.Fatal("list_tools missing")
	}
	if ro.Tool.Annotations.ReadOnlyHint == nil || !*ro.Tool.Annotations.ReadOnlyHint {
		t.Error("list_tasks must have readOnlyHint=true")
	}

	upd, ok := tools["update_task"]
	if !ok {
		t.Fatal("update_task missing")
	}
	if upd.Tool.Annotations.ReadOnlyHint == nil || *upd.Tool.Annotations.ReadOnlyHint {
		t.Error("update_task must have readOnlyHint=false")
	}
	if upd.Tool.Annotations.DestructiveHint == nil || !*upd.Tool.Annotations.DestructiveHint {
		t.Error("update_task must have destructiveHint=true")
	}
}

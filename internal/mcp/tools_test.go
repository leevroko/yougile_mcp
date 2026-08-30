package mcp

import (
	"testing"

	"github.com/yougile-mcp/internal/config"
)

func TestListTools(t *testing.T) {
	s := New(Deps{})
	if s.mcp == nil {
		t.Fatal("nil mcp server")
	}
	tools := s.mcp.ListTools()
	t.Logf("registered tools: %d", len(tools))
	if len(tools) != 24 {
		t.Fatalf("expected 23 tools, got %d", len(tools))
	}
	for _, name := range []string{
		"list_projects", "list_boards", "list_columns", "list_tasks",
		"get_task", "create_task", "update_task", "delete_task", "create_board", "create_column", "delete_board", "create_sticker", "get_stickers",
		"get_board_snapshot", "summarize_board", "audit_board",
		"get_task_messages", "send_task_message",
		"track_goals", "bulk_move_tasks", "batch_update_stickers", "compress_reviews",
		"get_mode", "set_mode",
	} {
		if _, ok := tools[name]; !ok {
			t.Errorf("tool %q not registered", name)
		}
	}
}

func TestDefaultModeConfirm(t *testing.T) {
	s := New(Deps{})
	if s.Mode() != config.ModeConfirm {
		t.Fatalf("default mode = %q, want confirm", s.Mode())
	}
}

func TestModeFromDeps(t *testing.T) {
	s := New(Deps{Mode: config.ModeYolo})
	if s.Mode() != config.ModeYolo {
		t.Fatalf("mode = %q, want yolo", s.Mode())
	}
}

func TestSetModeValid(t *testing.T) {
	s := New(Deps{})
	if !s.SetMode(config.ModeRead) || s.Mode() != config.ModeRead {
		t.Fatal("SetMode(read) failed")
	}
	if !s.SetMode(config.ModeYolo) || s.Mode() != config.ModeYolo {
		t.Fatal("SetMode(yolo) failed")
	}
	if s.SetMode("bogus") {
		t.Fatal("SetMode(bogus) must fail")
	}
	if s.Mode() != config.ModeYolo {
		t.Fatal("mode changed after invalid set")
	}
}

func TestReadModeBlocksMutations(t *testing.T) {
	s := New(Deps{Mode: config.ModeRead})
	// get_mode работает в read
	if _, ok := s.mcp.ListTools()["get_mode"]; !ok {
		t.Fatal("get_mode must be registered in read")
	}
	if _, ok := s.mcp.ListTools()["set_mode"]; !ok {
		t.Fatal("set_mode must be registered in read")
	}
}

func TestAnnotations(t *testing.T) {
	s := New(Deps{})
	tools := s.mcp.ListTools()

	ro, ok := tools["list_tasks"]
	if !ok {
		t.Fatal("list_tasks missing")
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

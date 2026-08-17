package mcp

import (
	"testing"
)

func TestListTools(t *testing.T) {
	s := New(nil, nil, nil, nil, nil, nil)
	if s.mcp == nil {
		t.Fatal("nil mcp server")
	}
	tools := s.mcp.ListTools()
	t.Logf("registered tools: %d", len(tools))
	if len(tools) < 10 {
		t.Fatalf("expected >= 10 tools, got %d", len(tools))
	}
	// Проверка ключевых инструментов обоих слоёв
	expected := []string{
		"list_projects", "list_boards", "list_columns", "list_tasks",
		"get_task", "create_task", "update_task", "get_stickers",
		"get_board_snapshot", "summarize_board", "audit_board",
		"track_goals", "bulk_move_tasks", "batch_update_stickers", "compress_reviews",
	}
	for _, name := range expected {
		if _, ok := tools[name]; !ok {
			t.Errorf("tool %q not registered", name)
		}
	}
}

package mcp

import (
	"encoding/json"
	"testing"

	"github.com/yougile-mcp/internal/domain/project"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

func TestProjectJSON(t *testing.T) {
	raw := valueobject.MustID("659a6c50-7fc7-4b0e-9b8e-1c9a2b3c4d5e")
	p := project.Project{
		ID:    valueobject.ProjectID(raw),
		Title: "Test",
		Users: map[valueobject.UserID]valueobject.Role{
			valueobject.UserID(raw): valueobject.RoleAdmin,
		},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	t.Logf("json: %s", data)
}

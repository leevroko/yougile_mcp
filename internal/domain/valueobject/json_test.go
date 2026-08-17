package valueobject

import (
	"encoding/json"
	"testing"
)

func TestIDJSON(t *testing.T) {
	id := MustID("659a6c50-7fc7-4b0e-9b8e-1c9a2b3c4d5e")
	data, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("marshal ID: %v", err)
	}
	if string(data) != `"659a6c50-7fc7-4b0e-9b8e-1c9a2b3c4d5e"` {
		t.Fatalf("unexpected: %s", data)
	}
}

func TestTypedIDJSON(t *testing.T) {
	id := MustID("659a6c50-7fc7-4b0e-9b8e-1c9a2b3c4d5e")
	pid := ProjectID(id)
	data, err := json.Marshal(pid)
	if err != nil {
		t.Fatalf("marshal ProjectID: %v", err)
	}
	if string(data) != `"659a6c50-7fc7-4b0e-9b8e-1c9a2b3c4d5e"` {
		t.Fatalf("unexpected: %s", data)
	}
}

func TestMapWithTypedKeyJSON(t *testing.T) {
	id := MustID("659a6c50-7fc7-4b0e-9b8e-1c9a2b3c4d5e")
	m := map[UserID]Role{UserID(id): RoleAdmin}
	// Go не сериализует map с ключами-структурами напрямую;
	// entity (Task/Project) используют кастомный MarshalJSON.
	if _, err := json.Marshal(m); err == nil {
		t.Fatal("expected error for struct-keyed map")
	}
}

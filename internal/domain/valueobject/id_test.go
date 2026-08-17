package valueobject

import (
	"testing"
)

func TestNewID_Valid(t *testing.T) {
	id, err := NewID("659a6c50-7fc7-4b0e-9b8e-1c9a2b3c4d5e")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.String() != "659a6c50-7fc7-4b0e-9b8e-1c9a2b3c4d5e" {
		t.Fatalf("wrong value: %s", id.String())
	}
}

func TestNewID_Invalid(t *testing.T) {
	cases := []string{
		"",
		"not-a-uuid",
		"659a6c50-7fc7-4b0e-9b8e",               // короткий
		"659a6c50-7fc7-4b0e-9b8e-1c9a2b3c4d5e1", // длинный
	}
	for _, c := range cases {
		_, err := NewID(c)
		if err == nil {
			t.Errorf("expected error for %q, got nil", c)
		}
	}
}

func TestID_IsZero(t *testing.T) {
	var id ID
	if !id.IsZero() {
		t.Fatal("zero ID should be IsZero")
	}
}

func TestMustID_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid ID")
		}
	}()
	MustID("bad")
}

func TestTypedIDs(t *testing.T) {
	bid, err := NewBoardID("659a6c50-7fc7-4b0e-9b8e-1c9a2b3c4d5e")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bid.String() != "659a6c50-7fc7-4b0e-9b8e-1c9a2b3c4d5e" {
		t.Fatalf("wrong value: %s", bid.String())
	}
	if bid.IsZero() {
		t.Fatal("valid BoardID should not be IsZero")
	}
}

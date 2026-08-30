package valueobject

import (
	"encoding/json"
	"testing"
)

func TestNewDeadline_Invalid(t *testing.T) {
	if _, err := NewDeadline(0); err == nil {
		t.Fatal("expected error for zero deadline")
	}
	if _, err := NewDeadline(-100); err == nil {
		t.Fatal("expected error for negative deadline")
	}
}

func TestDeadline_IsOverdue(t *testing.T) {
	now := int64(1_700_000_000_000) // ms

	d, err := NewDeadline(now - 1000) // прошёл час назад
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.IsOverdue(now) {
		t.Fatal("past deadline should be overdue")
	}

	d2, err := NewDeadline(now + 1000) // в будущем
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d2.IsOverdue(now) {
		t.Fatal("future deadline should not be overdue")
	}
}

func TestDeadline_OverdueDays(t *testing.T) {
	now := int64(1_700_000_000_000)
	day := int64(86400 * 1000)

	tests := []struct {
		name     string
		deadline int64
		want     int
	}{
		{"not overdue", now + day, 0},
		{"1 day", now - day, 1},
		{"2.5 days -> 3", now - (day*2 + day/2), 3},
		{"exactly 2 days", now - day*2, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewDeadline(tt.deadline)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := d.OverdueDays(now); got != tt.want {
				t.Fatalf("OverdueDays() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDeadlineMarshalJSONWireFormat(t *testing.T) {
	dl, err := NewDeadline(1756684800000)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(dl)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"deadline":1756684800000}` {
		t.Fatalf(" MarshalJSON: %s", b)
	}
	// nil-указатель → null (дедлайн не задан)
	var p *Deadline
	b, _ = json.Marshal(p)
	if string(b) != "null" {
		t.Fatalf("nil deadline: %s", b)
	}
	// UnmarshalJSON — симметрия
	var back Deadline
	if err := json.Unmarshal(b[:0], &back); err == nil {
		t.Fatal("empty unmarshal must fail")
	}
	if err := json.Unmarshal([]byte(`{"deadline":1756684800000}`), &back); err != nil {
		t.Fatal(err)
	}
	if back.Value() != dl.Value() {
		t.Fatalf("roundtrip: %d != %d", back.Value(), dl.Value())
	}
}

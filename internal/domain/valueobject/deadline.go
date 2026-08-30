package valueobject

import (
	"encoding/json"
	"errors"
)

// ErrInvalidDeadline — некорректный дедлайн (timestamp <= 0).
var ErrInvalidDeadline = errors.New("invalid deadline: timestamp must be positive")

// Deadline — дедлайн задачи (timestamp в миллисекундах).
// Используется сценариями Audit/Summarize для расчёта просрочки.
type Deadline struct {
	value int64 // timestamp ms
}

// NewDeadline создаёт Deadline из timestamp в миллисекундах.
// Значение <= 0 возвращает ошибку.
func NewDeadline(ms int64) (Deadline, error) {
	if ms <= 0 {
		return Deadline{}, ErrInvalidDeadline
	}
	return Deadline{value: ms}, nil
}

// IsZero возвращает true, если дедлайн не задан.
func (d Deadline) IsZero() bool { return d.value == 0 }

// Value возвращает timestamp в миллисекундах.
func (d Deadline) Value() int64 { return d.value }

// IsOverdue возвращает true, если дедлайн раньше nowMS.
func (d Deadline) IsOverdue(nowMS int64) bool {
	return d.value < nowMS
}

// MarshalJSON отдаёт дедлайн в формате API YouGile: {"deadline": <ms>}.
// Без него структура с приватным полем сериализуется как {} — get_task
 // показывал пустой дедлайн при реально записанном значении (issue #8).
func (d Deadline) MarshalJSON() ([]byte, error) {
	return json.Marshal(deadlineWire{Deadline: d.value})
}

// UnmarshalJSON читает {"deadline": <ms>} (симметрия MarshalJSON).
func (d *Deadline) UnmarshalJSON(b []byte) error {
	var w deadlineWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	d.value = w.Deadline
	return nil
}

type deadlineWire struct {
	Deadline int64 `json:"deadline"`
}

// OverdueDays возвращает количество полных дней просрочки (>= 0).
func (d Deadline) OverdueDays(nowMS int64) int {
	if !d.IsOverdue(nowMS) {
		return 0
	}
	diff := nowMS - d.value
	days := diff / (86400 * 1000)
	if diff%(86400*1000) != 0 {
		days++
	}
	return int(days)
}

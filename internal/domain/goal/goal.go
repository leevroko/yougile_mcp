// Package goal — домен целей с ключевыми результатами (KR).
package goal

import (
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// Goal — цель с ключевыми результатами (KR).
// Сценарий Goal Tracking: weighted average KR.
type Goal struct {
	// Значения из кастомных стикеров доски:
	// Client/Goal (string) — название цели
	// Weight (number) — вес KR
	// Progress (number) — прогресс 0-100
	Name     string // значение стикера Client/Goal
	Weight   int    // значение стикера Weight
	Progress int    // значение стикера Progress (0-100)

	Tasks []TaskRef // задачи, относящиеся к этой цели
}

// TaskRef — ссылка на задачу с её вкладом в цель.
type TaskRef struct {
	TaskID   valueobject.TaskID
	Title    string
	Weight   int
	Progress int
}

// Aggregate — все цели доски + их KR.
type Aggregate struct {
	Goals []Goal
}

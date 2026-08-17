package valueobject

// ColumnColor — числовой код цвета колонки (по API: 1 = #7B869E).
type ColumnColor int

// Константы цветов колонок (по API).
const (
	ColumnColorDefault ColumnColor = 1
	ColumnColorGray    ColumnColor = 2
	ColumnColorBlue    ColumnColor = 3
	ColumnColorGreen   ColumnColor = 4
	ColumnColorOrange  ColumnColor = 5
	ColumnColorRed     ColumnColor = 6
	ColumnColorPurple  ColumnColor = 7
)

// TaskColor — строковый класс цвета карточки задачи.
type TaskColor string

// Цвета карточек задач (по API).
const (
	TaskColorPrimary TaskColor = "task-primary"
	TaskColorGray    TaskColor = "task-gray"
	TaskColorRed     TaskColor = "task-red"
	TaskColorPink    TaskColor = "task-pink"
	TaskColorYellow  TaskColor = "task-yellow"
)

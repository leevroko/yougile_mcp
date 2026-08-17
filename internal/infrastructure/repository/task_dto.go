package repository

import (
	"strconv"

	"github.com/yougile-mcp/internal/domain/task"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// taskDTO — задача из ответа API.
type taskDTO struct {
	ID                 string         `json:"id"`
	Title              string         `json:"title"`
	Timestamp          int64          `json:"timestamp"`
	ColumnID           *string        `json:"columnId"`
	Description        string         `json:"description"`
	Completed          bool           `json:"completed"`
	CompletedTimestamp *int64         `json:"completedTimestamp"`
	Archived           bool           `json:"archived"`
	ArchivedTimestamp  *int64         `json:"archivedTimestamp"`
	Deleted            bool           `json:"deleted"`
	Assigned           []string       `json:"assigned"`
	CreatedBy          *string        `json:"createdBy"`
	Stickers           map[string]any `json:"stickers"`
	Deadline           *deadlineDTO   `json:"deadline"`
	Subtasks           []string       `json:"subtasks"`
	Color              string         `json:"color"`
}

// deadlineDTO — дедлайн задачи.
type deadlineDTO struct {
	Deadline int64 `json:"deadline"`
}

// taskListDTO — ответ списка задач.
type taskListDTO struct {
	Paging  pagingDTO `json:"paging"`
	Content []taskDTO `json:"content"`
}

// createTaskDTO — тело POST /tasks.
type createTaskDTO struct {
	Title       string         `json:"title"`
	ColumnID    *string        `json:"columnId,omitempty"`
	Description string         `json:"description,omitempty"`
	Deadline    *deadlineDTO   `json:"deadline,omitempty"`
	Completed   bool           `json:"completed,omitempty"`
	Assigned    []string       `json:"assigned,omitempty"`
	Stickers    map[string]any `json:"stickers,omitempty"`
	Subtasks    []string       `json:"subtasks,omitempty"`
	Color       string         `json:"color,omitempty"`
}

// updateTaskDTO — тело PUT /tasks/{id}.
type updateTaskDTO struct {
	Title       *string        `json:"title,omitempty"`
	ColumnID    *string        `json:"columnId,omitempty"`
	Description *string        `json:"description,omitempty"`
	Deadline    *deadlineDTO   `json:"deadline,omitempty"`
	Completed   *bool          `json:"completed,omitempty"`
	Archived    *bool          `json:"archived,omitempty"`
	Deleted     *bool          `json:"deleted,omitempty"`
	Assigned    []string       `json:"assigned,omitempty"`
	Stickers    map[string]any `json:"stickers,omitempty"`
	Subtasks    []string       `json:"subtasks,omitempty"`
	Color       *string        `json:"color,omitempty"`
}

// toDomain преобразует DTO в доменную задачу.
func (d taskDTO) toDomain() (task.Task, error) {
	id, err := valueobject.NewTaskID(d.ID)
	if err != nil {
		return task.Task{}, err
	}
	var columnID *valueobject.ColumnID
	if d.ColumnID != nil && *d.ColumnID != "-" {
		cid, err := valueobject.NewColumnID(*d.ColumnID)
		if err != nil {
			return task.Task{}, err
		}
		columnID = &cid
	}
	stickers := make(map[valueobject.StickerID]valueobject.StickerValue, len(d.Stickers))
	for k, v := range d.Stickers {
		sid, err := valueobject.NewStickerID(k)
		if err != nil {
			continue // пропустить некорректные ID
		}
		stickers[sid] = valueobject.StickerValue{Value: anyToString(v)}
	}
	var deadline *valueobject.Deadline
	if d.Deadline != nil && d.Deadline.Deadline > 0 {
		dl, err := valueobject.NewDeadline(d.Deadline.Deadline)
		if err == nil {
			deadline = &dl
		}
	}
	var createdBy *valueobject.UserID
	if d.CreatedBy != nil {
		uid, err := valueobject.NewUserID(*d.CreatedBy)
		if err == nil {
			createdBy = &uid
		}
	}
	assigned := make([]valueobject.UserID, 0, len(d.Assigned))
	for _, a := range d.Assigned {
		uid, err := valueobject.NewUserID(a)
		if err == nil {
			assigned = append(assigned, uid)
		}
	}
	subtasks := make([]valueobject.TaskID, 0, len(d.Subtasks))
	for _, s := range d.Subtasks {
		tid, err := valueobject.NewTaskID(s)
		if err == nil {
			subtasks = append(subtasks, tid)
		}
	}
	return task.Task{
		ID:                 id,
		Title:              d.Title,
		Timestamp:          d.Timestamp,
		ColumnID:           columnID,
		Description:        d.Description,
		Completed:          d.Completed,
		CompletedTimestamp: d.CompletedTimestamp,
		Archived:           d.Archived,
		ArchivedTimestamp:  d.ArchivedTimestamp,
		Deleted:            d.Deleted,
		Assigned:           assigned,
		CreatedBy:          createdBy,
		Stickers:           stickers,
		Deadline:           deadline,
		Subtasks:           subtasks,
		Color:              valueobject.TaskColor(d.Color),
	}, nil
}

// anyToString конвертирует значение JSON в строку.
func anyToString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return ""
	default:
		return ""
	}
}

// ptrString возвращает *string из ID (или nil).
func ptrString(id fmtStringer) *string {
	if id == nil || id.String() == "" {
		return nil
	}
	v := id.String()
	return &v
}

// idStrings конвертирует []ID в []string.
func idStrings[T fmtStringer](ids []T) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

// stickerValues конвертирует map[StickerID]StickerValue в map[string]any.
func stickerValues(m map[valueobject.StickerID]valueobject.StickerValue) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k.String()] = v.Value
	}
	return out
}

// fmtStringer — минимальный интерфейс для ID-типов.
type fmtStringer interface {
	String() string
}

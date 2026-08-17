// Package task — домен задач YouGile.
package task

import (
	"context"
	"encoding/json"

	"github.com/yougile-mcp/internal/domain/valueobject"
)

// Task — задача (карточка). Задействована во всех сценариях.
type Task struct {
	ID                 valueobject.TaskID
	Title              string
	Timestamp          int64
	ColumnID           *valueobject.ColumnID // nil = без колонки
	Description        string
	Completed          bool
	CompletedTimestamp *int64
	Archived           bool
	ArchivedTimestamp  *int64
	Deleted            bool
	Assigned           []valueobject.UserID
	CreatedBy          *valueobject.UserID
	Stickers           map[valueobject.StickerID]valueobject.StickerValue
	Deadline           *valueobject.Deadline
	Subtasks           []valueobject.TaskID
	Color              valueobject.TaskColor
}

// IsOverdue возвращает true, если задача просрочена (deadline < nowMS).
func (t Task) IsOverdue(nowMS int64) bool {
	return t.Deadline != nil && t.Deadline.IsOverdue(nowMS)
}

// MarshalJSON сериализует Task, приводя ключи стикеров к строкам.
func (t Task) MarshalJSON() ([]byte, error) {
	stickers := make(map[string]valueobject.StickerValue, len(t.Stickers))
	for k, v := range t.Stickers {
		stickers[k.String()] = v
	}
	assigned := make([]string, 0, len(t.Assigned))
	for _, a := range t.Assigned {
		assigned = append(assigned, a.String())
	}
	subtasks := make([]string, 0, len(t.Subtasks))
	for _, s := range t.Subtasks {
		subtasks = append(subtasks, s.String())
	}
	return json.Marshal(struct {
		ID                 string                              `json:"id"`
		Title              string                              `json:"title"`
		Timestamp          int64                               `json:"timestamp"`
		ColumnID           *string                             `json:"columnId"`
		Description        string                              `json:"description"`
		Completed          bool                                `json:"completed"`
		CompletedTimestamp *int64                              `json:"completedTimestamp"`
		Archived           bool                                `json:"archived"`
		ArchivedTimestamp  *int64                              `json:"archivedTimestamp"`
		Deleted            bool                                `json:"deleted"`
		Assigned           []string                            `json:"assigned"`
		CreatedBy          *string                             `json:"createdBy"`
		Stickers           map[string]valueobject.StickerValue `json:"stickers"`
		Deadline           *valueobject.Deadline               `json:"deadline"`
		Subtasks           []string                            `json:"subtasks"`
		Color              string                              `json:"color"`
	}{
		ID:                 t.ID.String(),
		Title:              t.Title,
		Timestamp:          t.Timestamp,
		ColumnID:           idPtr(t.ColumnID),
		Description:        t.Description,
		Completed:          t.Completed,
		CompletedTimestamp: t.CompletedTimestamp,
		Archived:           t.Archived,
		ArchivedTimestamp:  t.ArchivedTimestamp,
		Deleted:            t.Deleted,
		Assigned:           assigned,
		CreatedBy:          idPtr(t.CreatedBy),
		Stickers:           stickers,
		Deadline:           t.Deadline,
		Subtasks:           subtasks,
		Color:              string(t.Color),
	})
}

// idPtr конвертирует *ID в *string.
func idPtr[T interface{ String() string }](p *T) *string {
	if p == nil {
		return nil
	}
	s := (*p).String()
	return &s
}

// StickerValue возвращает значение стикера и флаг наличия.
func (t Task) StickerValue(id valueobject.StickerID) (valueobject.StickerValue, bool) {
	v, ok := t.Stickers[id]
	return v, ok
}

// Repository — интерфейс доступа к задачам.
type Repository interface {
	// List возвращает задачи по фильтру с пагинацией.
	List(ctx context.Context, filter Filter) ([]Task, valueobject.PagingMetadata, error)
	// GetByID возвращает задачу по ID; ErrNotFound если нет.
	GetByID(ctx context.Context, id valueobject.TaskID) (Task, error)
	// Create создаёт задачу, возвращает ID.
	Create(ctx context.Context, req CreateRequest) (valueobject.TaskID, error)
	// Update обновляет задачу (частично).
	Update(ctx context.Context, id valueobject.TaskID, req UpdateRequest) error
	// Delete — мягкое удаление (deleted=true), как в API.
	Delete(ctx context.Context, id valueobject.TaskID) error
}

// Filter — фильтр списка задач.
type Filter struct {
	BoardID  *valueobject.BoardID  // без columnId — все задачи доски (неопределённость №6)
	ColumnID *valueobject.ColumnID // конкретная колонка
	Limit    int
	Offset   int
}

// CreateRequest — создание задачи.
type CreateRequest struct {
	Title       string
	ColumnID    *valueobject.ColumnID
	Description string
	Deadline    *valueobject.Deadline
	Completed   bool
	Assigned    []valueobject.UserID
	Stickers    map[valueobject.StickerID]valueobject.StickerValue
	Subtasks    []valueobject.TaskID
	Color       valueobject.TaskColor
}

// UpdateRequest — частичное обновление: только переданные поля (Quick Actions).
type UpdateRequest struct {
	Title       *string
	ColumnID    *valueobject.ColumnID // nil = снять с колонки
	Description *string
	Deadline    *valueobject.Deadline
	Completed   *bool
	Archived    *bool
	Deleted     *bool
	Assigned    *[]valueobject.UserID
	Stickers    *map[valueobject.StickerID]valueobject.StickerValue
	Subtasks    *[]valueobject.TaskID
	Color       *valueobject.TaskColor
}

// Aggregate — задача с подзадачами.
type Aggregate struct {
	Task     Task
	Subtasks []Task // подзадачи (по Subtasks IDs)
}

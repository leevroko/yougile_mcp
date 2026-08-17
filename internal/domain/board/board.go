// Package board — домен досок YouGile.
package board

import (
	"context"

	"github.com/yougile-mcp/internal/domain/task"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// Board — доска (Kanban).
type Board struct {
	ID        valueobject.BoardID
	Title     string
	ProjectID valueobject.ProjectID
	Stickers  []StickerSetting // настройки кастомных стикеров доски
	Deleted   bool
}

// StickerSetting — настройка одного кастомного стикера доски.
type StickerSetting struct {
	StickerID valueobject.StickerID
	Title     string
	Type      valueobject.StickerType
	Options   []StickerOption // для select: {id, title, color}
}

// StickerOption — опция select-стикера.
type StickerOption struct {
	ID    valueobject.StateID
	Title string
	Color *string // hex
}

// Repository — интерфейс доступа к доскам.
type Repository interface {
	// List возвращает доски проекта с пагинацией.
	List(ctx context.Context, projectID valueobject.ProjectID) ([]Board, valueobject.PagingMetadata, error)
	// GetByID возвращает доску по ID; ErrNotFound если нет.
	GetByID(ctx context.Context, id valueobject.BoardID) (Board, error)
	// Create создаёт доску, возвращает ID.
	Create(ctx context.Context, req CreateRequest) (valueobject.BoardID, error)
	// Update обновляет доску (частично).
	Update(ctx context.Context, id valueobject.BoardID, req UpdateRequest) error
}

// CreateRequest — создание доски.
type CreateRequest struct {
	Title     string
	ProjectID valueobject.ProjectID
	Stickers  []StickerSetting
}

// UpdateRequest — обновление доски; nil-поля не изменяются.
type UpdateRequest struct {
	Title     *string
	ProjectID *valueobject.ProjectID
	Deleted   *bool
}

// Aggregate — доска с колонками, задачами и легендой стикеров.
// Используется сценариями Summarize/Audit/Goal Tracking (snapshot).
type Aggregate struct {
	Board    Board
	Columns  []Column
	Tasks    []task.Task
	Stickers []Sticker // легенда для расшифровки
}

// Column — колонка доски (в домене board для Aggregate).
type Column struct {
	ID      valueobject.ColumnID
	Title   string
	BoardID valueobject.BoardID
	Color   valueobject.ColumnColor
	Deleted bool
}

// Sticker — легенда стикера (в домене board для Aggregate).
type Sticker struct {
	ID      valueobject.StickerID
	Title   string
	Type    valueobject.StickerType
	Options []StickerOption
}

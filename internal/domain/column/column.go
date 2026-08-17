// Package column — домен колонок доски.
package column

import (
	"context"

	"github.com/yougile-mcp/internal/domain/valueobject"
)

// Column — статусная колонка доски.
type Column struct {
	ID      valueobject.ColumnID
	Title   string
	BoardID valueobject.BoardID
	Color   valueobject.ColumnColor
	Deleted bool
}

// Repository — интерфейс доступа к колонкам.
type Repository interface {
	// List возвращает колонки доски.
	List(ctx context.Context, boardID valueobject.BoardID) ([]Column, valueobject.PagingMetadata, error)
	// GetByID возвращает колонку по ID; ErrNotFound если нет.
	GetByID(ctx context.Context, id valueobject.ColumnID) (Column, error)
	// Create создаёт колонку, возвращает ID.
	Create(ctx context.Context, req CreateRequest) (valueobject.ColumnID, error)
	// Update обновляет колонку (частично).
	Update(ctx context.Context, id valueobject.ColumnID, req UpdateRequest) error
}

// CreateRequest — создание колонки.
type CreateRequest struct {
	Title   string
	BoardID valueobject.BoardID
	Color   valueobject.ColumnColor
}

// UpdateRequest — обновление колонки; nil-поля не изменяются.
type UpdateRequest struct {
	Title   *string
	BoardID *valueobject.BoardID
	Color   *valueobject.ColumnColor
	Deleted *bool
}

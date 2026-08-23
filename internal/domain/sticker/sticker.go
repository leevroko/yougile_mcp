// Package sticker — домен стикеров: кастомные (старые), string и sprint.
package sticker

import (
	"context"

	"github.com/yougile-mcp/internal/domain/valueobject"
)

// Sticker — кастомный стикер доски (старый /stickers).
type Sticker struct {
	ID      valueobject.StickerID
	Title   string
	Type    valueobject.StickerType
	BoardID valueobject.BoardID
	Options []StickerOption // для select
}

// StickerOption — опция select-стикера.
type StickerOption struct {
	ID    valueobject.StateID
	Title string
	Color *string
}

// Repository — интерфейс доступа к кастомным стикерам.
type Repository interface {
	// List возвращает стикеры доски.
	List(ctx context.Context, boardID valueobject.BoardID) ([]Sticker, error)
	// GetByID возвращает стикер по ID; ErrNotFound если нет.
	GetByID(ctx context.Context, id valueobject.StickerID) (Sticker, error)
	// Create создаёт стикер, возвращает ID.
	Create(ctx context.Context, req CreateRequest) (valueobject.StickerID, error)
	// Update обновляет стикер (частично).
	Update(ctx context.Context, id valueobject.StickerID, req UpdateRequest) error
	// AttachToBoard привязывает стикер к доске (merge в stickers.custom).
	AttachToBoard(ctx context.Context, id valueobject.StickerID, boardID valueobject.BoardID) error
}

// CreateRequest — создание стикера (/string-stickers).
// Title → name, Options → states; BoardID (если задан) — привязать к доске
// после создания (merge в stickers.custom). Type игнорируется: все
// string-стикеры select-подобны.
type CreateRequest struct {
	Title   string
	Icon    string
	Type    valueobject.StickerType
	BoardID valueobject.BoardID
	Options []StickerOption
}

// UpdateRequest — обновление стикера; nil-поля не изменяются.
type UpdateRequest struct {
	Title   *string
	Type    *valueobject.StickerType
	Options *[]StickerOption
}

// StringSticker — новый механизм: dropdown-стикер с состояниями.
type StringSticker struct {
	ID      valueobject.StringStickerID
	Name    string
	Icon    string
	States  []StringStickerState
	Deleted bool
}

// StringStickerState — состояние string-стикера.
type StringStickerState struct {
	ID      valueobject.StateID
	Name    string
	Color   *string
	Deleted bool
}

// SprintSticker — спринт-стикер (с датами begin/end в СЕКУНДАХ).
type SprintSticker struct {
	ID      valueobject.SprintStickerID
	Name    string
	States  []SprintStickerState
	Deleted bool
}

// SprintStickerState — состояние спринт-стикера.
type SprintStickerState struct {
	ID      valueobject.StateID
	Name    string
	Begin   int64 // секунды, НЕ ms (отличие от deadline!)
	End     int64
	Deleted bool
}

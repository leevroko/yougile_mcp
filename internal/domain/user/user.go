// Package user — домен пользователей YouGile.
package user

import (
	"context"

	"github.com/yougile-mcp/internal/domain/valueobject"
)

// User — пользователь компании.
type User struct {
	ID           valueobject.UserID
	Email        string
	RealName     string
	Status       string // online / offline
	LastActivity int64
	IsAdmin      bool
}

// Repository — интерфейс доступа к пользователям.
type Repository interface {
	// List возвращает пользователей с пагинацией.
	List(ctx context.Context) ([]User, valueobject.PagingMetadata, error)
	// GetByID возвращает пользователя по ID; ErrNotFound если нет.
	GetByID(ctx context.Context, id valueobject.UserID) (User, error)
	// GetMe возвращает текущего пользователя.
	GetMe(ctx context.Context) (User, error)
	// Create приглашает пользователя, возвращает ID.
	Create(ctx context.Context, req CreateRequest) (valueobject.UserID, error)
	// Update обновляет пользователя (частично).
	Update(ctx context.Context, id valueobject.UserID, req UpdateRequest) error
}

// CreateRequest — приглашение пользователя.
type CreateRequest struct {
	Email   string
	IsAdmin bool
}

// UpdateRequest — обновление пользователя; nil-поля не изменяются.
type UpdateRequest struct {
	IsAdmin *bool
}

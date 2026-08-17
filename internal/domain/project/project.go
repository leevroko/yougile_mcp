// Package project — домен проектов YouGile.
package project

import (
	"context"
	"encoding/json"

	"github.com/yougile-mcp/internal/domain/valueobject"
)

// Project — проект (группа досок).
type Project struct {
	ID        valueobject.ProjectID
	Title     string
	Timestamp int64
	Users     map[valueobject.UserID]valueobject.Role // userId: role
	Deleted   bool
}

// MarshalJSON сериализует Project, приводя ключи Users к строкам.
func (p Project) MarshalJSON() ([]byte, error) {
	users := make(map[string]valueobject.Role, len(p.Users))
	for k, v := range p.Users {
		users[k.String()] = v
	}
	return json.Marshal(struct {
		ID        string                      `json:"id"`
		Title     string                      `json:"title"`
		Timestamp int64                       `json:"timestamp"`
		Users     map[string]valueobject.Role `json:"users"`
		Deleted   bool                        `json:"deleted"`
	}{
		ID:        p.ID.String(),
		Title:     p.Title,
		Timestamp: p.Timestamp,
		Users:     users,
		Deleted:   p.Deleted,
	})
}

// Repository — интерфейс доступа к проектам.
type Repository interface {
	// List возвращает проекты с пагинацией.
	List(ctx context.Context) ([]Project, valueobject.PagingMetadata, error)
	// GetByID возвращает проект по ID; ErrNotFound если нет.
	GetByID(ctx context.Context, id valueobject.ProjectID) (Project, error)
	// Create создаёт проект, возвращает ID.
	Create(ctx context.Context, req CreateRequest) (valueobject.ProjectID, error)
	// Update обновляет проект (частично).
	Update(ctx context.Context, id valueobject.ProjectID, req UpdateRequest) error
}

// CreateRequest — создание проекта.
type CreateRequest struct {
	Title string
	Users map[valueobject.UserID]valueobject.Role
}

// UpdateRequest — обновление проекта; nil-поля не изменяются.
type UpdateRequest struct {
	Title   *string
	Users   *map[valueobject.UserID]valueobject.Role
	Deleted *bool
}

package repository

import (
	"context"
	"net/http"

	"github.com/yougile-mcp/internal/domain/user"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// userRepository — HTTP-реализация user.Repository.
type userRepository struct {
	client *client
}

// NewUserRepository создаёт user.Repository.
func NewUserRepository(hc *http.Client, baseURL string) user.Repository {
	return &userRepository{client: newClient(hc, baseURL)}
}

// userDTO — пользователь из ответа API.
type userDTO struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	RealName     string `json:"realName"`
	Status       string `json:"status"`
	LastActivity int64  `json:"lastActivity"`
	IsAdmin      bool   `json:"isAdmin"`
}

// userListDTO — ответ списка пользователей.
type userListDTO struct {
	Paging  pagingDTO `json:"paging"`
	Content []userDTO `json:"content"`
}

func (d userDTO) toDomain() (user.User, error) {
	id, err := valueobject.NewUserID(d.ID)
	if err != nil {
		return user.User{}, err
	}
	return user.User{
		ID:           id,
		Email:        d.Email,
		RealName:     d.RealName,
		Status:       d.Status,
		LastActivity: d.LastActivity,
		IsAdmin:      d.IsAdmin,
	}, nil
}

// List возвращает пользователей.
func (r *userRepository) List(ctx context.Context) ([]user.User, valueobject.PagingMetadata, error) {
	var dto userListDTO
	if err := r.client.get(ctx, "/users", &dto); err != nil {
		return nil, valueobject.PagingMetadata{}, err
	}
	out := make([]user.User, 0, len(dto.Content))
	for _, u := range dto.Content {
		uv, err := u.toDomain()
		if err != nil {
			return nil, valueobject.PagingMetadata{}, err
		}
		out = append(out, uv)
	}
	return out, valueobject.PagingMetadata{
		Count: dto.Paging.Count, Limit: dto.Paging.Limit,
		Offset: dto.Paging.Offset, Next: dto.Paging.Next,
	}, nil
}

// GetByID возвращает пользователя по ID.
func (r *userRepository) GetByID(ctx context.Context, id valueobject.UserID) (user.User, error) {
	var dto userDTO
	if err := r.client.get(ctx, "/users/"+id.String(), &dto); err != nil {
		return user.User{}, err
	}
	return dto.toDomain()
}

// GetMe возвращает текущего пользователя.
func (r *userRepository) GetMe(ctx context.Context) (user.User, error) {
	var dto userDTO
	if err := r.client.get(ctx, "/users/me", &dto); err != nil {
		return user.User{}, err
	}
	return dto.toDomain()
}

// Create приглашает пользователя.
func (r *userRepository) Create(ctx context.Context, req user.CreateRequest) (valueobject.UserID, error) {
	body := map[string]any{"email": req.Email, "isAdmin": req.IsAdmin}
	var out struct {
		ID string `json:"id"`
	}
	if err := r.client.post(ctx, "/users", body, &out); err != nil {
		return valueobject.UserID{}, err
	}
	id, err := valueobject.NewUserID(out.ID)
	if err != nil {
		return valueobject.UserID{}, err
	}
	return id, nil
}

// Update обновляет пользователя (частично).
func (r *userRepository) Update(ctx context.Context, id valueobject.UserID, req user.UpdateRequest) error {
	body := map[string]any{}
	if req.IsAdmin != nil {
		body["isAdmin"] = *req.IsAdmin
	}
	return r.client.put(ctx, "/users/"+id.String(), body, nil)
}

package repository

import (
	"context"
	"net/http"

	"github.com/yougile-mcp/internal/domain/project"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// projectRepository — HTTP-реализация project.Repository.
type projectRepository struct {
	client *client
}

// NewProjectRepository создаёт project.Repository.
func NewProjectRepository(hc *http.Client, baseURL string) project.Repository {
	return &projectRepository{client: newClient(hc, baseURL)}
}

// projectDTO — проект из ответа API.
type projectDTO struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Timestamp int64             `json:"timestamp"`
	Users     map[string]string `json:"users"` // userId: role
	Deleted   bool              `json:"deleted"`
}

// projectListDTO — ответ списка проектов.
type projectListDTO struct {
	Paging  pagingDTO    `json:"paging"`
	Content []projectDTO `json:"content"`
}

func (d projectDTO) toDomain() (project.Project, error) {
	id, err := valueobject.NewProjectID(d.ID)
	if err != nil {
		return project.Project{}, err
	}
	users := make(map[valueobject.UserID]valueobject.Role, len(d.Users))
	for uid, role := range d.Users {
		u, err := valueobject.NewUserID(uid)
		if err != nil {
			continue
		}
		users[u] = valueobject.Role(role)
	}
	return project.Project{
		ID:        id,
		Title:     d.Title,
		Timestamp: d.Timestamp,
		Users:     users,
		Deleted:   d.Deleted,
	}, nil
}

// List возвращает проекты.
func (r *projectRepository) List(ctx context.Context) ([]project.Project, valueobject.PagingMetadata, error) {
	var dto projectListDTO
	if err := r.client.get(ctx, "/projects", &dto); err != nil {
		return nil, valueobject.PagingMetadata{}, err
	}
	out := make([]project.Project, 0, len(dto.Content))
	for _, p := range dto.Content {
		pv, err := p.toDomain()
		if err != nil {
			return nil, valueobject.PagingMetadata{}, err
		}
		out = append(out, pv)
	}
	return out, valueobject.PagingMetadata{
		Count: dto.Paging.Count, Limit: dto.Paging.Limit,
		Offset: dto.Paging.Offset, Next: dto.Paging.Next,
	}, nil
}

// GetByID возвращает проект по ID.
func (r *projectRepository) GetByID(ctx context.Context, id valueobject.ProjectID) (project.Project, error) {
	var dto projectDTO
	if err := r.client.get(ctx, "/projects/"+id.String(), &dto); err != nil {
		return project.Project{}, err
	}
	return dto.toDomain()
}

// Create создаёт проект.
func (r *projectRepository) Create(ctx context.Context, req project.CreateRequest) (valueobject.ProjectID, error) {
	body := map[string]any{"title": req.Title}
	if len(req.Users) > 0 {
		users := make(map[string]string, len(req.Users))
		for uid, role := range req.Users {
			users[uid.String()] = string(role)
		}
		body["users"] = users
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := r.client.post(ctx, "/projects", body, &out); err != nil {
		return valueobject.ProjectID{}, err
	}
	id, err := valueobject.NewProjectID(out.ID)
	if err != nil {
		return valueobject.ProjectID{}, err
	}
	return id, nil
}

// Update обновляет проект (частично).
func (r *projectRepository) Update(ctx context.Context, id valueobject.ProjectID, req project.UpdateRequest) error {
	body := map[string]any{}
	if req.Title != nil {
		body["title"] = *req.Title
	}
	if req.Users != nil {
		users := make(map[string]string, len(*req.Users))
		for uid, role := range *req.Users {
			users[uid.String()] = string(role)
		}
		body["users"] = users
	}
	if req.Deleted != nil {
		body["deleted"] = *req.Deleted
	}
	return r.client.put(ctx, "/projects/"+id.String(), body, nil)
}

package repository

import (
	"context"
	"net/http"

	"github.com/yougile-mcp/internal/domain/board"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// boardRepository — HTTP-реализация board.Repository.
type boardRepository struct {
	client *client
}

// NewBoardRepository создаёт board.Repository.
func NewBoardRepository(hc *http.Client, baseURL string) board.Repository {
	return &boardRepository{client: newClient(hc, baseURL)}
}

// boardDTO — доска из ответа API.
type boardDTO struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	ProjectID string          `json:"projectId"`
	Stickers  stickersSetting `json:"stickers"`
	Deleted   bool            `json:"deleted"`
}

// stickersSetting — настройки стикеров доски (формат custom не специфицирован).
type stickersSetting struct {
	Custom map[string]any `json:"custom"`
}

// boardListDTO — ответ списка досок.
type boardListDTO struct {
	Paging  pagingDTO  `json:"paging"`
	Content []boardDTO `json:"content"`
}

func (d boardDTO) toDomain() (board.Board, error) {
	id, err := valueobject.NewBoardID(d.ID)
	if err != nil {
		return board.Board{}, err
	}
	pid, err := valueobject.NewProjectID(d.ProjectID)
	if err != nil {
		return board.Board{}, err
	}
	return board.Board{
		ID:        id,
		Title:     d.Title,
		ProjectID: pid,
		Deleted:   d.Deleted,
	}, nil
}

// List возвращает доски проекта.
func (r *boardRepository) List(ctx context.Context, projectID valueobject.ProjectID) ([]board.Board, valueobject.PagingMetadata, error) {
	path := addQuery("/boards", map[string]string{"projectId": projectID.String()})
	var dto boardListDTO
	if err := r.client.get(ctx, path, &dto); err != nil {
		return nil, valueobject.PagingMetadata{}, err
	}
	out := make([]board.Board, 0, len(dto.Content))
	for _, b := range dto.Content {
		bv, err := b.toDomain()
		if err != nil {
			return nil, valueobject.PagingMetadata{}, err
		}
		out = append(out, bv)
	}
	return out, valueobject.PagingMetadata{
		Count: dto.Paging.Count, Limit: dto.Paging.Limit,
		Offset: dto.Paging.Offset, Next: dto.Paging.Next,
	}, nil
}

// GetByID возвращает доску по ID.
func (r *boardRepository) GetByID(ctx context.Context, id valueobject.BoardID) (board.Board, error) {
	var dto boardDTO
	if err := r.client.get(ctx, "/boards/"+id.String(), &dto); err != nil {
		return board.Board{}, err
	}
	return dto.toDomain()
}

// Create создаёт доску.
func (r *boardRepository) Create(ctx context.Context, req board.CreateRequest) (valueobject.BoardID, error) {
	body := struct {
		Title     string `json:"title"`
		ProjectID string `json:"projectId"`
	}{req.Title, req.ProjectID.String()}
	var out struct {
		ID string `json:"id"`
	}
	if err := r.client.post(ctx, "/boards", body, &out); err != nil {
		return valueobject.BoardID{}, err
	}
	id, err := valueobject.NewBoardID(out.ID)
	if err != nil {
		return valueobject.BoardID{}, err
	}
	return id, nil
}

// Update обновляет доску (частично).
func (r *boardRepository) Update(ctx context.Context, id valueobject.BoardID, req board.UpdateRequest) error {
	body := map[string]any{}
	if req.Title != nil {
		body["title"] = *req.Title
	}
	if req.ProjectID != nil {
		body["projectId"] = req.ProjectID.String()
	}
	if req.Deleted != nil {
		body["deleted"] = *req.Deleted
	}
	return r.client.put(ctx, "/boards/"+id.String(), body, nil)
}

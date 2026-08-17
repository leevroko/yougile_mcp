package repository

import (
	"context"
	"net/http"

	"github.com/yougile-mcp/internal/domain/column"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// columnRepository — HTTP-реализация column.Repository.
type columnRepository struct {
	client *client
}

// NewColumnRepository создаёт column.Repository.
func NewColumnRepository(hc *http.Client, baseURL string) column.Repository {
	return &columnRepository{client: newClient(hc, baseURL)}
}

// columnDTO — колонка из ответа API.
type columnDTO struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	BoardID string `json:"boardId"`
	Color   int    `json:"color"`
	Deleted bool   `json:"deleted"`
}

// columnListDTO — ответ списка колонок.
type columnListDTO struct {
	Paging  pagingDTO   `json:"paging"`
	Content []columnDTO `json:"content"`
}

func (d columnDTO) toDomain() (column.Column, error) {
	id, err := valueobject.NewColumnID(d.ID)
	if err != nil {
		return column.Column{}, err
	}
	bid, err := valueobject.NewBoardID(d.BoardID)
	if err != nil {
		return column.Column{}, err
	}
	return column.Column{
		ID:      id,
		Title:   d.Title,
		BoardID: bid,
		Color:   valueobject.ColumnColor(d.Color),
		Deleted: d.Deleted,
	}, nil
}

// List возвращает колонки доски.
func (r *columnRepository) List(ctx context.Context, boardID valueobject.BoardID) ([]column.Column, valueobject.PagingMetadata, error) {
	path := addQuery("/columns", map[string]string{"boardId": boardID.String()})
	var dto columnListDTO
	if err := r.client.get(ctx, path, &dto); err != nil {
		return nil, valueobject.PagingMetadata{}, err
	}
	out := make([]column.Column, 0, len(dto.Content))
	for _, c := range dto.Content {
		cv, err := c.toDomain()
		if err != nil {
			return nil, valueobject.PagingMetadata{}, err
		}
		out = append(out, cv)
	}
	return out, valueobject.PagingMetadata{
		Count: dto.Paging.Count, Limit: dto.Paging.Limit,
		Offset: dto.Paging.Offset, Next: dto.Paging.Next,
	}, nil
}

// GetByID возвращает колонку по ID.
func (r *columnRepository) GetByID(ctx context.Context, id valueobject.ColumnID) (column.Column, error) {
	var dto columnDTO
	if err := r.client.get(ctx, "/columns/"+id.String(), &dto); err != nil {
		return column.Column{}, err
	}
	return dto.toDomain()
}

// Create создаёт колонку.
func (r *columnRepository) Create(ctx context.Context, req column.CreateRequest) (valueobject.ColumnID, error) {
	body := struct {
		Title   string `json:"title"`
		BoardID string `json:"boardId"`
		Color   int    `json:"color"`
	}{req.Title, req.BoardID.String(), int(req.Color)}
	var out struct {
		ID string `json:"id"`
	}
	if err := r.client.post(ctx, "/columns", body, &out); err != nil {
		return valueobject.ColumnID{}, err
	}
	id, err := valueobject.NewColumnID(out.ID)
	if err != nil {
		return valueobject.ColumnID{}, err
	}
	return id, nil
}

// Update обновляет колонку (частично).
func (r *columnRepository) Update(ctx context.Context, id valueobject.ColumnID, req column.UpdateRequest) error {
	body := map[string]any{}
	if req.Title != nil {
		body["title"] = *req.Title
	}
	if req.BoardID != nil {
		body["boardId"] = req.BoardID.String()
	}
	if req.Color != nil {
		body["color"] = int(*req.Color)
	}
	if req.Deleted != nil {
		body["deleted"] = *req.Deleted
	}
	return r.client.put(ctx, "/columns/"+id.String(), body, nil)
}

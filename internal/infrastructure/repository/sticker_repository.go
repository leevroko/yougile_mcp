package repository

import (
	"context"
	"net/http"

	"github.com/yougile-mcp/internal/domain/sticker"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// stickerRepository — HTTP-реализация sticker.Repository (старые /stickers).
type stickerRepository struct {
	client *client
}

// NewStickerRepository создаёт sticker.Repository.
func NewStickerRepository(hc *http.Client, baseURL string) sticker.Repository {
	return &stickerRepository{client: newClient(hc, baseURL)}
}

// stickerDTO — кастомный стикер из ответа API.
type stickerDTO struct {
	ID      string          `json:"id"`
	Title   string          `json:"title"`
	Type    string          `json:"type"`
	BoardID string          `json:"boardId"`
	Options []stickerOption `json:"options"`
}

// stickerOption — опция select-стикера.
type stickerOption struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Color *string `json:"color"`
}

func (d stickerDTO) toDomain() (sticker.Sticker, error) {
	id, err := valueobject.NewStickerID(d.ID)
	if err != nil {
		return sticker.Sticker{}, err
	}
	bid, err := valueobject.NewBoardID(d.BoardID)
	if err != nil {
		return sticker.Sticker{}, err
	}
	opts := make([]sticker.StickerOption, 0, len(d.Options))
	for _, o := range d.Options {
		oid, err := valueobject.NewStateID(o.ID)
		if err != nil {
			continue
		}
		opts = append(opts, sticker.StickerOption{ID: oid, Title: o.Title, Color: o.Color})
	}
	return sticker.Sticker{
		ID:      id,
		Title:   d.Title,
		Type:    valueobject.StickerType(d.Type),
		BoardID: bid,
		Options: opts,
	}, nil
}

// List возвращает стикеры доски (массив, без paging).
func (r *stickerRepository) List(ctx context.Context, boardID valueobject.BoardID) ([]sticker.Sticker, error) {
	path := addQuery("/stickers", map[string]string{"boardId": boardID.String()})
	var dto []stickerDTO
	if err := r.client.get(ctx, path, &dto); err != nil {
		return nil, err
	}
	out := make([]sticker.Sticker, 0, len(dto))
	for _, s := range dto {
		sv, err := s.toDomain()
		if err != nil {
			return nil, err
		}
		out = append(out, sv)
	}
	return out, nil
}

// GetByID возвращает стикер по ID.
func (r *stickerRepository) GetByID(ctx context.Context, id valueobject.StickerID) (sticker.Sticker, error) {
	var dto stickerDTO
	if err := r.client.get(ctx, "/stickers/"+id.String(), &dto); err != nil {
		return sticker.Sticker{}, err
	}
	return dto.toDomain()
}

// Create создаёт стикер.
func (r *stickerRepository) Create(ctx context.Context, req sticker.CreateRequest) (valueobject.StickerID, error) {
	body := map[string]any{
		"title":   req.Title,
		"type":    string(req.Type),
		"boardId": req.BoardID.String(),
	}
	if len(req.Options) > 0 {
		opts := make([]map[string]any, 0, len(req.Options))
		for _, o := range req.Options {
			m := map[string]any{"title": o.Title}
			if o.Color != nil {
				m["color"] = *o.Color
			}
			opts = append(opts, m)
		}
		body["options"] = opts
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := r.client.post(ctx, "/stickers", body, &out); err != nil {
		return valueobject.StickerID{}, err
	}
	id, err := valueobject.NewStickerID(out.ID)
	if err != nil {
		return valueobject.StickerID{}, err
	}
	return id, nil
}

// Update обновляет стикер (частично).
func (r *stickerRepository) Update(ctx context.Context, id valueobject.StickerID, req sticker.UpdateRequest) error {
	body := map[string]any{}
	if req.Title != nil {
		body["title"] = *req.Title
	}
	if req.Type != nil {
		body["type"] = string(*req.Type)
	}
	if req.Options != nil {
		opts := make([]map[string]any, 0, len(*req.Options))
		for _, o := range *req.Options {
			m := map[string]any{"title": o.Title}
			if o.Color != nil {
				m["color"] = *o.Color
			}
			opts = append(opts, m)
		}
		body["options"] = opts
	}
	return r.client.put(ctx, "/stickers/"+id.String(), body, nil)
}

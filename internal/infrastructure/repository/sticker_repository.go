package repository

import (
	"context"
	"errors"
	"net/http"

	"github.com/yougile-mcp/internal/domain/domainerr"
	"github.com/yougile-mcp/internal/domain/sticker"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// Алиасы доменных ошибок.
var (
	ErrNotFound     = domainerr.ErrNotFound
	ErrNotSupported = errors.New("not supported for string-stickers")
)

// stickerRepository — HTTP-реализация sticker.Repository.
// Легенда стикеров берётся из /string-stickers (новый механизм) +
// фильтруется по stickers.custom из /boards/{id}.
type stickerRepository struct {
	client *client
}

// NewStickerRepository создаёт sticker.Repository.
func NewStickerRepository(hc *http.Client, baseURL string) sticker.Repository {
	return &stickerRepository{client: newClient(hc, baseURL)}
}

// stringStickerDTO — стикер из /string-stickers.
type stringStickerDTO struct {
	ID      string                  `json:"id"`
	Name    string                  `json:"name"`
	Icon    string                  `json:"icon"`
	States  []stringStickerStateDTO `json:"states"`
	Deleted *bool                   `json:"deleted"`
}

// stringStickerStateDTO — состояние string-стикера.
type stringStickerStateDTO struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Color   *string `json:"color"`
	Deleted *bool   `json:"deleted"`
}

// stringStickerListDTO — ответ /string-stickers.
type stringStickerListDTO struct {
	Paging  pagingDTO          `json:"paging"`
	Content []stringStickerDTO `json:"content"`
}

// createStringStickerDTO — тело POST /string-stickers.
type createStringStickerDTO struct {
	Name   string                      `json:"name"`
	Icon   string                      `json:"icon,omitempty"`
	States []stringStickerStateNoIDDTO `json:"states"`
}

// stringStickerStateNoIDDTO — состояние без ID (при создании).
type stringStickerStateNoIDDTO struct {
	Name  string  `json:"name"`
	Color *string `json:"color,omitempty"`
}

// boardStickersDTO — поле stickers из /boards/{id}.
type boardStickersDTO struct {
	Stickers struct {
		Deadline bool            `json:"deadline"`
		Custom   map[string]bool `json:"custom"`
	} `json:"stickers"`
}

// List возвращает стикеры доски (только кастомные из stickers.custom).
func (r *stickerRepository) List(ctx context.Context, boardID valueobject.BoardID) ([]sticker.Sticker, error) {
	// 1. Получить ID кастомных стикеров доски из /boards/{id}
	var boardStickers boardStickersDTO
	if err := r.client.get(ctx, "/boards/"+boardID.String(), &boardStickers); err != nil {
		return nil, err
	}
	customIDs := boardStickers.Stickers.Custom

	// 2. Получить все string-стикеры
	var dto stringStickerListDTO
	if err := r.client.get(ctx, "/string-stickers?limit=100", &dto); err != nil {
		return nil, err
	}

	// 3. Отфильтровать по custom ID и смапить
	out := make([]sticker.Sticker, 0)
	for _, s := range dto.Content {
		if s.ID == "" || s.Deleted != nil && *s.Deleted {
			continue
		}
		// Включить только если стикер принадлежит доске (есть в custom) ИЛИ custom пуст (неизвестно)
		if len(customIDs) > 0 {
			if _, ok := customIDs[s.ID]; !ok {
				continue
			}
		}
		sid, err := valueobject.NewStickerID(s.ID)
		if err != nil {
			continue
		}
		st := sticker.Sticker{
			ID:      sid,
			Title:   s.Name,
			Type:    valueobject.StickerTypeSelect,
			BoardID: boardID,
		}
		for _, state := range s.States {
			if state.ID == "" || state.Deleted != nil && *state.Deleted {
				continue
			}
			sid2, err := valueobject.NewStateID(state.ID)
			if err != nil {
				continue
			}
			st.Options = append(st.Options, sticker.StickerOption{
				ID:    sid2,
				Title: state.Name,
				Color: state.Color,
			})
		}
		out = append(out, st)
	}
	return out, nil
}

// GetByID возвращает стикер по ID (поиск по всем).
func (r *stickerRepository) GetByID(ctx context.Context, id valueobject.StickerID) (sticker.Sticker, error) {
	all, err := r.listAll(ctx)
	if err != nil {
		return sticker.Sticker{}, err
	}
	for _, s := range all {
		if s.ID == id {
			return s, nil
		}
	}
	return sticker.Sticker{}, ErrNotFound
}

// Create — POST /string-stickers {name, icon, states}. BoardID игнорируется
// (привязка — отдельным вызовом AttachToBoard).
func (r *stickerRepository) Create(ctx context.Context, req sticker.CreateRequest) (valueobject.StickerID, error) {
	if req.Title == "" {
		return valueobject.StickerID{}, errors.New("sticker name is required")
	}
	states := make([]stringStickerStateNoIDDTO, 0, len(req.Options))
	for _, o := range req.Options {
		if o.Title == "" {
			continue
		}
		states = append(states, stringStickerStateNoIDDTO{Name: o.Title, Color: o.Color})
	}
	body := createStringStickerDTO{Name: req.Title, Icon: req.Icon, States: states}
	var out struct {
		ID string `json:"id"`
	}
	if err := r.client.post(ctx, "/string-stickers", body, &out); err != nil {
		return valueobject.StickerID{}, err
	}
	id, err := valueobject.NewStickerID(out.ID)
	if err != nil {
		return valueobject.StickerID{}, err
	}
	return id, nil
}

// AttachToBoard привязывает стикер к доске: GET текущего stickers → merge
// custom → PUT /boards/{id}. Перезапись чужих custom-записей исключена.
func (r *stickerRepository) AttachToBoard(ctx context.Context, id valueobject.StickerID, boardID valueobject.BoardID) error {
	var cur boardStickersDTO
	if err := r.client.get(ctx, "/boards/"+boardID.String(), &cur); err != nil {
		return err
	}
	custom := cur.Stickers.Custom
	if custom == nil {
		custom = map[string]bool{}
	}
	if custom[id.String()] {
		return nil // уже привязан
	}
	custom[id.String()] = true
	body := boardStickersDTO{}
	body.Stickers.Custom = custom
	if cur.Stickers.Deadline {
		body.Stickers.Deadline = true
	}
	return r.client.put(ctx, "/boards/"+boardID.String(), body, nil)
}

// Update — не поддерживается для /string-stickers.
func (r *stickerRepository) Update(ctx context.Context, id valueobject.StickerID, req sticker.UpdateRequest) error {
	return ErrNotSupported
}

// listAll возвращает все string-стикеры (без фильтра по доске).
func (r *stickerRepository) listAll(ctx context.Context) ([]sticker.Sticker, error) {
	var dto stringStickerListDTO
	if err := r.client.get(ctx, "/string-stickers?limit=100", &dto); err != nil {
		return nil, err
	}
	out := make([]sticker.Sticker, 0, len(dto.Content))
	for _, s := range dto.Content {
		if s.ID == "" || s.Deleted != nil && *s.Deleted {
			continue
		}
		sid, err := valueobject.NewStickerID(s.ID)
		if err != nil {
			continue
		}
		st := sticker.Sticker{ID: sid, Title: s.Name, Type: valueobject.StickerTypeSelect}
		for _, state := range s.States {
			if state.ID == "" {
				continue
			}
			sid2, err := valueobject.NewStateID(state.ID)
			if err != nil {
				continue
			}
			st.Options = append(st.Options, sticker.StickerOption{ID: sid2, Title: state.Name, Color: state.Color})
		}
		out = append(out, st)
	}
	return out, nil
}

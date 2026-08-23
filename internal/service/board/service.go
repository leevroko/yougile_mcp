// Package board — доменный сервис досок.
package board

import (
	"context"
	"sync"
	"time"

	"github.com/yougile-mcp/internal/domain/board"
	"github.com/yougile-mcp/internal/domain/column"
	"github.com/yougile-mcp/internal/domain/project"
	"github.com/yougile-mcp/internal/domain/sticker"
	"github.com/yougile-mcp/internal/domain/task"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// Service — доменный сервис досок: CRUD + snapshot.
type Service interface {
	ListProjects(ctx context.Context) ([]project.Project, valueobject.PagingMetadata, error)
	ListBoards(ctx context.Context, projectID valueobject.ProjectID) ([]board.Board, valueobject.PagingMetadata, error)
	ListColumns(ctx context.Context, boardID valueobject.BoardID) ([]column.Column, error)
	ListStickers(ctx context.Context, boardID valueobject.BoardID) ([]sticker.Sticker, error)

	// GetBoardSnapshot — полное состояние доски (сценарий snapshot).
	// По умолчанию только активные задачи (не completed, не archived);
	// includeCompleted/includeArchived включают их.
	GetBoardSnapshot(ctx context.Context, boardID valueobject.BoardID, since *int64) (board.Aggregate, error)
	GetBoardSnapshotFiltered(ctx context.Context, boardID valueobject.BoardID, since *int64, f SnapshotFilter) (board.Aggregate, error)

	CreateProject(ctx context.Context, title string) (valueobject.ProjectID, error)
	CreateBoard(ctx context.Context, title string, projectID valueobject.ProjectID) (valueobject.BoardID, error)
	CreateColumn(ctx context.Context, title string, boardID valueobject.BoardID, color valueobject.ColumnColor) (valueobject.ColumnID, error)

	// DeleteBoard — мягкое удаление доски (deleted=true), как в API.
	DeleteBoard(ctx context.Context, boardID valueobject.BoardID) error

	// CreateSticker — создать string-стикер (states = набор состояний);
	// при заданном boardID — привязать его к доске.
	CreateSticker(ctx context.Context, req sticker.CreateRequest) (valueobject.StickerID, bool, error)
}

// NewService создаёт BoardService.
func NewService(
	projects project.Repository,
	boards board.Repository,
	columns column.Repository,
	stickers sticker.Repository,
	tasks task.Repository,
) Service {
	return &service{projects: projects, boards: boards, columns: columns, stickers: stickers, tasks: tasks}
}

type service struct {
	projects project.Repository
	boards   board.Repository
	columns  column.Repository
	stickers sticker.Repository
	tasks    task.Repository
}

func (s *service) ListProjects(ctx context.Context) ([]project.Project, valueobject.PagingMetadata, error) {
	return s.projects.List(ctx)
}

func (s *service) ListBoards(ctx context.Context, projectID valueobject.ProjectID) ([]board.Board, valueobject.PagingMetadata, error) {
	return s.boards.List(ctx, projectID)
}

func (s *service) ListColumns(ctx context.Context, boardID valueobject.BoardID) ([]column.Column, error) {
	cols, _, err := s.columns.List(ctx, boardID)
	return cols, err
}

func (s *service) ListStickers(ctx context.Context, boardID valueobject.BoardID) ([]sticker.Sticker, error) {
	return s.stickers.List(ctx, boardID)
}

// SnapshotFilter — фильтрация задач в snapshot.
type SnapshotFilter struct {
	IncludeCompleted bool
	IncludeArchived  bool
}

// GetBoardSnapshot собирает агрегат: доска + колонки + задачи + легенда стикеров.
// По умолчанию только активные задачи.
func (s *service) GetBoardSnapshot(ctx context.Context, boardID valueobject.BoardID, since *int64) (board.Aggregate, error) {
	return s.GetBoardSnapshotFiltered(ctx, boardID, since, SnapshotFilter{})
}

// snapshotFetchConcurrency — сколько колонок фетчим параллельно (issue #2).
const snapshotFetchConcurrency = 4

// snapshotTimeout — общий дедлайн снапшота (issue #2). Меньше 60s-таймаута
// MCP-клиента, чтобы клиент получил внятную ошибку вместо разрыва соединения.
// var (не const) — чтобы тесты могли переопределить.
var snapshotTimeout = 45 * time.Second

// GetBoardSnapshotFiltered — snapshot с фильтрацией задач.
func (s *service) GetBoardSnapshotFiltered(ctx context.Context, boardID valueobject.BoardID, since *int64, f SnapshotFilter) (board.Aggregate, error) {
	ctx, cancel := context.WithTimeout(ctx, snapshotTimeout)
	defer cancel()

	b, err := s.boards.GetByID(ctx, boardID)
	if err != nil {
		return board.Aggregate{}, err
	}

	cols, _, err := s.columns.List(ctx, boardID)
	if err != nil {
		return board.Aggregate{}, err
	}

	// Полный обход задач: по каждой колонке (API требует columnId — неопределённость №6
	// подтверждена: GET /tasks не принимает boardId, возвращает 400).
	// Колонки фетчим параллельно (bounded, issue #2); страницы внутри колонки
	// последовательны. Результаты пишем в perCol[i] — порядок колонок сохраняется.
	perCol := make([][]task.Task, len(cols))
	errs := make([]error, len(cols))
	sem := make(chan struct{}, snapshotFetchConcurrency)
	var wg sync.WaitGroup
	for i, c := range cols {
		wg.Add(1)
		go func(i int, colID valueobject.ColumnID) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			out := make([]task.Task, 0)
			offset := 0
			for {
				filter := task.Filter{BoardID: &boardID, ColumnID: &colID, Limit: 100, Offset: offset}
				tasks, paging, err := s.tasks.List(ctx, filter)
				if err != nil {
					errs[i] = err
					return
				}
				for _, t := range tasks {
					if since != nil && t.Timestamp < *since {
						continue // дельта: пропустить не изменившиеся
					}
					if t.Deleted {
						continue
					}
					if t.Archived && !f.IncludeArchived {
						continue
					}
					if t.Completed && !f.IncludeCompleted {
						continue
					}
					out = append(out, t)
				}
				if !paging.HasNext() || len(tasks) == 0 {
					break
				}
				offset = paging.Offset + paging.Limit
			}
			perCol[i] = out
		}(i, c.ID)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return board.Aggregate{}, err
		}
	}
	all := make([]task.Task, 0)
	for _, tasks := range perCol {
		all = append(all, tasks...)
	}

	st, err := s.stickers.List(ctx, boardID)
	if err != nil {
		return board.Aggregate{}, err
	}

	// Маппинг колонок в домен board (агрегат)
	colsDomain := make([]board.Column, 0, len(cols))
	for _, c := range cols {
		colsDomain = append(colsDomain, board.Column{
			ID: c.ID, Title: c.Title, BoardID: c.BoardID, Color: c.Color, Deleted: c.Deleted,
		})
	}
	stDomain := make([]board.Sticker, 0, len(st))
	for _, s2 := range st {
		opts := make([]board.StickerOption, 0, len(s2.Options))
		for _, o := range s2.Options {
			opts = append(opts, board.StickerOption{ID: o.ID, Title: o.Title, Color: o.Color})
		}
		stDomain = append(stDomain, board.Sticker{ID: s2.ID, Title: s2.Title, Type: s2.Type, Options: opts})
	}

	return board.Aggregate{
		Board:    b,
		Columns:  colsDomain,
		Tasks:    all, // доменные task.Task (с IsOverdue)
		Stickers: stDomain,
	}, nil
}

func (s *service) CreateProject(ctx context.Context, title string) (valueobject.ProjectID, error) {
	return s.projects.Create(ctx, project.CreateRequest{Title: title})
}

func (s *service) CreateBoard(ctx context.Context, title string, projectID valueobject.ProjectID) (valueobject.BoardID, error) {
	return s.boards.Create(ctx, board.CreateRequest{Title: title, ProjectID: projectID})
}

func (s *service) CreateColumn(ctx context.Context, title string, boardID valueobject.BoardID, color valueobject.ColumnColor) (valueobject.ColumnID, error) {
	return s.columns.Create(ctx, column.CreateRequest{Title: title, BoardID: boardID, Color: color})
}

// DeleteBoard — мягкое удаление (deleted=true). Задачи/колонки остаются в API,
// но доска исчезает из списков и снапшотов.
func (s *service) DeleteBoard(ctx context.Context, boardID valueobject.BoardID) error {
	deleted := true
	return s.boards.Update(ctx, boardID, board.UpdateRequest{Deleted: &deleted})
}

// CreateSticker создаёт string-стикер; второй результат — привязан ли к доске.
func (s *service) CreateSticker(ctx context.Context, req sticker.CreateRequest) (valueobject.StickerID, bool, error) {
	sid, err := s.stickers.Create(ctx, req)
	if err != nil {
		return valueobject.StickerID{}, false, err
	}
	attached := false
	if !req.BoardID.IsZero() {
		if err := s.stickers.AttachToBoard(ctx, sid, req.BoardID); err != nil {
			return sid, false, err
		}
		attached = true
	}
	return sid, attached, nil
}

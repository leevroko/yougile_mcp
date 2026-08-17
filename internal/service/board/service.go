// Package board — доменный сервис досок.
package board

import (
	"context"

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
	// Обходит все колонки и все страницы задач.
	GetBoardSnapshot(ctx context.Context, boardID valueobject.BoardID, since *int64) (board.Aggregate, error)

	CreateProject(ctx context.Context, title string) (valueobject.ProjectID, error)
	CreateBoard(ctx context.Context, title string, projectID valueobject.ProjectID) (valueobject.BoardID, error)
	CreateColumn(ctx context.Context, title string, boardID valueobject.BoardID, color valueobject.ColumnColor) (valueobject.ColumnID, error)
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

// GetBoardSnapshot собирает агрегат: доска + колонки + задачи + легенда стикеров.
func (s *service) GetBoardSnapshot(ctx context.Context, boardID valueobject.BoardID, since *int64) (board.Aggregate, error) {
	b, err := s.boards.GetByID(ctx, boardID)
	if err != nil {
		return board.Aggregate{}, err
	}

	cols, _, err := s.columns.List(ctx, boardID)
	if err != nil {
		return board.Aggregate{}, err
	}

	// Полный обход задач доски (без columnId — все колонки сразу).
	all := make([]task.Task, 0)
	offset := 0
	for {
		filter := task.Filter{BoardID: &boardID, Limit: 100, Offset: offset}
		tasks, paging, err := s.tasks.List(ctx, filter)
		if err != nil {
			return board.Aggregate{}, err
		}
		for _, t := range tasks {
			if since != nil && t.Timestamp < *since {
				continue // дельта: пропустить не изменившиеся
			}
			all = append(all, t)
		}
		if !paging.HasNext() || len(tasks) == 0 {
			break
		}
		offset = paging.Offset + paging.Limit
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

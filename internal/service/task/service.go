// Package task — доменный сервис задач.
package task

import (
	"context"
	"errors"
	"fmt"
	"sync"

	domaincolumn "github.com/yougile-mcp/internal/domain/column"
	"github.com/yougile-mcp/internal/domain/domainerr"
	domainsticker "github.com/yougile-mcp/internal/domain/sticker"
	domaintask "github.com/yougile-mcp/internal/domain/task"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// Service — доменный сервис задач: CRUD + массовые операции.
type Service interface {
	// Quick Actions
	CreateTask(ctx context.Context, req CreateTaskParams) (valueobject.TaskID, error)
	GetTask(ctx context.Context, taskID valueobject.TaskID) (domaintask.Task, error)
	UpdateTask(ctx context.Context, taskID valueobject.TaskID, req domaintask.UpdateRequest) error
	MoveTask(ctx context.Context, taskID valueobject.TaskID, columnID valueobject.ColumnID) error
	DeleteTask(ctx context.Context, taskID valueobject.TaskID) error

	// ListTasks — с вложенными справочными данными (колонки + легенда стикеров)
	ListTasks(ctx context.Context, boardID valueobject.BoardID, columnID *valueobject.ColumnID) (ListResult, error)

	// Bulk Move
	BulkMove(ctx context.Context, req BulkMoveParams) (BulkMoveResult, error)

	// Batch Stickers
	BatchUpdateStickers(ctx context.Context, req BatchStickersParams) (BatchStickersResult, error)
}

// CreateTaskParams — параметры создания задачи.
type CreateTaskParams struct {
	Title       string
	ColumnID    *valueobject.ColumnID
	Description string
	Deadline    *valueobject.Deadline
	Assigned    []valueobject.UserID
	Stickers    map[valueobject.StickerID]valueobject.StickerValue
}

// ListResult — задачи + справочные данные (для вложения в ответ).
type ListResult struct {
	Tasks      []domaintask.Task
	Columns    []domaincolumn.Column                           // названия колонок
	StickerMap map[valueobject.StickerID]domainsticker.Sticker // легенда
}

// BulkMoveParams — параметры массового перемещения.
type BulkMoveParams struct {
	BoardID        valueobject.BoardID
	SourceColumnID *valueobject.ColumnID
	TaskIDs        []valueobject.TaskID
	TargetColumnID valueobject.ColumnID
	DryRun         bool
}

// BulkMoveResult — результат массового перемещения.
type BulkMoveResult struct {
	Moved    int
	Failed   int
	NotFound int
	Details  []MoveDetail
}

// MoveDetail — деталь перемещения одной задачи.
type MoveDetail struct {
	TaskID valueobject.TaskID
	Status string // "moved" | "failed" | "not_found" | "already"
}

// BatchStickersParams — параметры массового обновления стикеров.
type BatchStickersParams struct {
	BoardID  valueobject.BoardID
	TaskIDs  []valueobject.TaskID
	Stickers map[valueobject.StickerID]valueobject.StickerValue
	DryRun   bool
}

// BatchStickersResult — результат массового обновления стикеров.
type BatchStickersResult struct {
	Updated  int
	Failed   int
	NotFound int
}

// NewService создаёт TaskService.
func NewService(tasks domaintask.Repository, columns domaincolumn.Repository, stickers domainsticker.Repository) Service {
	return &service{tasks: tasks, columns: columns, stickers: stickers}
}

type service struct {
	tasks    domaintask.Repository
	columns  domaincolumn.Repository
	stickers domainsticker.Repository
}

func (s *service) CreateTask(ctx context.Context, req CreateTaskParams) (valueobject.TaskID, error) {
	cr := domaintask.CreateRequest{
		Title:       req.Title,
		ColumnID:    req.ColumnID,
		Description: req.Description,
		Deadline:    req.Deadline,
		Assigned:    req.Assigned,
		Stickers:    req.Stickers,
	}
	return s.tasks.Create(ctx, cr)
}

func (s *service) GetTask(ctx context.Context, taskID valueobject.TaskID) (domaintask.Task, error) {
	return s.tasks.GetByID(ctx, taskID)
}

func (s *service) UpdateTask(ctx context.Context, taskID valueobject.TaskID, req domaintask.UpdateRequest) error {
	return s.tasks.Update(ctx, taskID, req)
}

func (s *service) MoveTask(ctx context.Context, taskID valueobject.TaskID, columnID valueobject.ColumnID) error {
	return s.tasks.Update(ctx, taskID, domaintask.UpdateRequest{ColumnID: &columnID})
}

func (s *service) DeleteTask(ctx context.Context, taskID valueobject.TaskID) error {
	return s.tasks.Delete(ctx, taskID)
}

// ListTasks возвращает задачи с вложенными колонками и легендой стикеров.
func (s *service) ListTasks(ctx context.Context, boardID valueobject.BoardID, columnID *valueobject.ColumnID) (ListResult, error) {
	filter := domaintask.Filter{BoardID: &boardID, ColumnID: columnID, Limit: 100}
	tasks, _, err := s.tasks.List(ctx, filter)
	if err != nil {
		return ListResult{}, err
	}

	cols, _, err := s.columns.List(ctx, boardID)
	if err != nil {
		return ListResult{}, err
	}

	st, err := s.stickers.List(ctx, boardID)
	if err != nil {
		return ListResult{}, err
	}
	stickerMap := make(map[valueobject.StickerID]domainsticker.Sticker, len(st))
	for _, s2 := range st {
		stickerMap[s2.ID] = s2
	}

	return ListResult{Tasks: tasks, Columns: cols, StickerMap: stickerMap}, nil
}

// BulkMove перемещает задачи в целевую колонку.
func (s *service) BulkMove(ctx context.Context, req BulkMoveParams) (BulkMoveResult, error) {
	var result BulkMoveResult

	// Определить задачи для перемещения
	taskIDs := req.TaskIDs
	if req.SourceColumnID != nil {
		filter := domaintask.Filter{BoardID: &req.BoardID, ColumnID: req.SourceColumnID, Limit: 100}
		srcTasks, _, err := s.tasks.List(ctx, filter)
		if err != nil {
			return result, err
		}
		taskIDs = make([]valueobject.TaskID, 0, len(srcTasks))
		for _, t := range srcTasks {
			taskIDs = append(taskIDs, t.ID)
		}
	}

	// Дедупликация
	seen := make(map[valueobject.TaskID]struct{}, len(taskIDs))
	unique := make([]valueobject.TaskID, 0, len(taskIDs))
	for _, id := range taskIDs {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}

	// Параллельные PUT с ограничением (rate limit)
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, id := range unique {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			detail := s.moveOne(ctx, id, req)
			mu.Lock()
			result.Details = append(result.Details, detail)
			switch detail.Status {
			case "moved":
				result.Moved++
			case "failed":
				result.Failed++
			case "not_found":
				result.NotFound++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return result, nil
}

// moveOne перемещает одну задачу.
func (s *service) moveOne(ctx context.Context, id valueobject.TaskID, req BulkMoveParams) MoveDetail {
	if req.DryRun {
		return MoveDetail{TaskID: id, Status: "moved"}
	}
	// идемпотентность: если уже в целевой — пропустить
	t, err := s.tasks.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domainerr.ErrNotFound) {
			return MoveDetail{TaskID: id, Status: "not_found"}
		}
		return MoveDetail{TaskID: id, Status: "failed"}
	}
	if t.ColumnID != nil && *t.ColumnID == req.TargetColumnID {
		return MoveDetail{TaskID: id, Status: "already"}
	}
	if err := s.tasks.Update(ctx, id, domaintask.UpdateRequest{ColumnID: &req.TargetColumnID}); err != nil {
		return MoveDetail{TaskID: id, Status: "failed"}
	}
	return MoveDetail{TaskID: id, Status: "moved"}
}

// BatchUpdateStickers проставляет стикеры группе задач.
func (s *service) BatchUpdateStickers(ctx context.Context, req BatchStickersParams) (BatchStickersResult, error) {
	var result BatchStickersResult

	// Валидация легенды стикеров до операции
	legend, err := s.stickers.List(ctx, req.BoardID)
	if err != nil {
		return result, err
	}
	valid := make(map[valueobject.StickerID]struct{}, len(legend))
	for _, st := range legend {
		valid[st.ID] = struct{}{}
	}
	for sid := range req.Stickers {
		if _, ok := valid[sid]; !ok {
			return result, fmt.Errorf("task: sticker %s not found on board", sid.String())
		}
	}

	if req.DryRun {
		result.Updated = len(req.TaskIDs)
		return result, nil
	}

	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, id := range req.TaskIDs {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := s.tasks.Update(ctx, id, domaintask.UpdateRequest{Stickers: &req.Stickers}); err != nil {
				mu.Lock()
				result.Failed++
				mu.Unlock()
				return
			}
			mu.Lock()
			result.Updated++
			mu.Unlock()
		}()
	}
	wg.Wait()
	return result, nil
}

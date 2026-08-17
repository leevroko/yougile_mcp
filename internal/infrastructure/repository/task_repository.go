package repository

import (
	"context"
	"net/http"
	"strconv"

	"github.com/yougile-mcp/internal/domain/task"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// taskRepository — HTTP-реализация task.Repository.
type taskRepository struct {
	client *client
}

// NewTaskRepository создаёт task.Repository.
func NewTaskRepository(hc *http.Client, baseURL string) task.Repository {
	return &taskRepository{client: newClient(hc, baseURL)}
}

// List возвращает задачи по фильтру, обходя пагинацию.
func (r *taskRepository) List(ctx context.Context, filter task.Filter) ([]task.Task, valueobject.PagingMetadata, error) {
	all := make([]task.Task, 0)
	var lastPaging valueobject.PagingMetadata

	params := map[string]string{"limit": "100"}
	if filter.BoardID != nil {
		params["boardId"] = filter.BoardID.String()
	}
	if filter.ColumnID != nil {
		params["columnId"] = filter.ColumnID.String()
	}

	offset := filter.Offset
	for {
		params["offset"] = strconv.Itoa(offset)
		path := addQuery("/tasks", params)

		var dto taskListDTO
		if err := r.client.get(ctx, path, &dto); err != nil {
			return nil, lastPaging, err
		}
		for _, td := range dto.Content {
			t, err := td.toDomain()
			if err != nil {
				return nil, lastPaging, err
			}
			all = append(all, t)
		}
		lastPaging = valueobject.PagingMetadata{
			Count:  dto.Paging.Count,
			Limit:  dto.Paging.Limit,
			Offset: dto.Paging.Offset,
			Next:   dto.Paging.Next,
		}
		if !dto.Paging.Next || len(dto.Content) == 0 {
			break
		}
		offset = dto.Paging.Offset + dto.Paging.Limit
	}
	return all, lastPaging, nil
}

// GetByID возвращает задачу по ID.
func (r *taskRepository) GetByID(ctx context.Context, id valueobject.TaskID) (task.Task, error) {
	var dto taskDTO
	if err := r.client.get(ctx, "/tasks/"+id.String(), &dto); err != nil {
		return task.Task{}, err
	}
	return dto.toDomain()
}

// Create создаёт задачу.
func (r *taskRepository) Create(ctx context.Context, req task.CreateRequest) (valueobject.TaskID, error) {
	body := createTaskDTO{
		Title:       req.Title,
		ColumnID:    ptrString(req.ColumnID),
		Description: req.Description,
		Completed:   req.Completed,
		Assigned:    idStrings(req.Assigned),
		Stickers:    stickerValues(req.Stickers),
		Subtasks:    idStrings(req.Subtasks),
		Color:       string(req.Color),
	}
	if req.Deadline != nil {
		body.Deadline = &deadlineDTO{Deadline: req.Deadline.Value()}
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := r.client.post(ctx, "/tasks", body, &out); err != nil {
		return valueobject.TaskID{}, err
	}
	id, err := valueobject.NewTaskID(out.ID)
	if err != nil {
		return valueobject.TaskID{}, err
	}
	return id, nil
}

// Update обновляет задачу (частично).
func (r *taskRepository) Update(ctx context.Context, id valueobject.TaskID, req task.UpdateRequest) error {
	body := updateTaskDTO{}
	if req.Title != nil {
		body.Title = req.Title
	}
	if req.ColumnID != nil {
		v := req.ColumnID.String()
		body.ColumnID = &v
	}
	if req.Description != nil {
		body.Description = req.Description
	}
	if req.Completed != nil {
		body.Completed = req.Completed
	}
	if req.Archived != nil {
		body.Archived = req.Archived
	}
	if req.Deleted != nil {
		body.Deleted = req.Deleted
	}
	if req.Assigned != nil {
		body.Assigned = idStrings(*req.Assigned)
	}
	if req.Stickers != nil {
		body.Stickers = stickerValues(*req.Stickers)
	}
	if req.Subtasks != nil {
		body.Subtasks = idStrings(*req.Subtasks)
	}
	if req.Color != nil {
		v := string(*req.Color)
		body.Color = &v
	}
	if req.Deadline != nil {
		body.Deadline = &deadlineDTO{Deadline: req.Deadline.Value()}
	}
	return r.client.put(ctx, "/tasks/"+id.String(), body, nil)
}

// Delete — мягкое удаление (deleted=true).
func (r *taskRepository) Delete(ctx context.Context, id valueobject.TaskID) error {
	deleted := true
	return r.client.put(ctx, "/tasks/"+id.String(), updateTaskDTO{Deleted: &deleted}, nil)
}

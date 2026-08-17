// Package audit — доменный сервис аудита доски.
package audit

import (
	"context"
	"strings"
	"time"

	"github.com/yougile-mcp/internal/domain/valueobject"
	boardservice "github.com/yougile-mcp/internal/service/board"
	taskservice "github.com/yougile-mcp/internal/service/task"
)

// Service — аудит доски (сценарий Audit).
type Service interface {
	Audit(ctx context.Context, req Params) (Result, error)
}

// Params — параметры аудита.
type Params struct {
	BoardID valueobject.BoardID
	Rules   Rules
	DryRun  bool
}

// Rules — какие проверки включить.
type Rules struct {
	Overdue         bool // просрочка
	MissingStickers bool // нет обязательных стикеров
	AutoMove        bool // перемещать просроченные в Review
}

// Result — результат аудита.
type Result struct {
	Issues              []Issue
	OverdueCount        int
	MissingStickerCount int
	AutoMoved           int
	DryRun              bool
}

// Issue — найденная проблема.
type Issue struct {
	Type        string // "overdue" | "missing_sticker"
	TaskID      valueobject.TaskID
	Title       string
	Description string
}

// NewService создаёт AuditService.
func NewService(boards boardservice.Service, tasks taskservice.Service) Service {
	return &service{boards: boards, tasks: tasks}
}

type service struct {
	boards boardservice.Service
	tasks  taskservice.Service
}

// requiredStickers — обязательные стикеры (конфиг-константа, как в легаси).
var requiredStickers = []string{"Weight", "Progress"}

// Audit проверяет доску по правилам.
func (s *service) Audit(ctx context.Context, req Params) (Result, error) {
	result := Result{DryRun: req.DryRun}

	snap, err := s.boards.GetBoardSnapshot(ctx, req.BoardID, nil)
	if err != nil {
		return result, err
	}

	// Найти колонку Review
	var reviewCol *valueobject.ColumnID
	for _, c := range snap.Columns {
		if c.Title == "Review" {
			reviewCol = &c.ID
			break
		}
	}

	now := time.Now().UnixMilli()

	for _, t := range snap.Tasks {
		if t.Deleted || t.Completed {
			continue
		}

		// Просрочка
		if req.Rules.Overdue && t.IsOverdue(now) {
			result.OverdueCount++
			result.Issues = append(result.Issues, Issue{
				Type:        "overdue",
				TaskID:      t.ID,
				Title:       t.Title,
				Description: "просрочена",
			})
		}

		// Отсутствие обязательных стикеров
		if req.Rules.MissingStickers && len(t.Stickers) == 0 {
			result.MissingStickerCount++
			result.Issues = append(result.Issues, Issue{
				Type:        "missing_sticker",
				TaskID:      t.ID,
				Title:       t.Title,
				Description: "нет стикеров: " + strings.Join(requiredStickers, ", "),
			})
		}

		// Авто-перемещение просроченных в Review
		if req.Rules.AutoMove && req.Rules.Overdue && t.IsOverdue(now) && !req.DryRun {
			if reviewCol == nil {
				continue // колонки Review нет — не перемещать
			}
			if t.ColumnID != nil && *t.ColumnID == *reviewCol {
				continue // уже в Review
			}
			if err := s.tasks.MoveTask(ctx, t.ID, *reviewCol); err == nil {
				result.AutoMoved++
			}
		}
	}

	return result, nil
}

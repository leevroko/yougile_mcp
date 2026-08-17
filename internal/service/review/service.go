// Package review — доменный сервис аналитики доски.
package review

import (
	"context"
	"strconv"
	"time"

	"github.com/yougile-mcp/internal/domain/board"
	"github.com/yougile-mcp/internal/domain/valueobject"
	boardservice "github.com/yougile-mcp/internal/service/board"
)

// Service — аналитика доски (сценарий Summarize).
type Service interface {
	Summarize(ctx context.Context, boardID valueobject.BoardID, since *int64) (Summary, error)
}

// Summary — сводка по доске.
type Summary struct {
	BoardID         valueobject.BoardID
	BoardTitle      string
	GeneratedAt     int64 // timestamp ms
	TotalTasks      int
	OverdueCount    int
	MissingSticker  int
	AvgProgress     float64
	ByColumn        []ColumnSummary
	ByGoal          []GoalSummary
	Recommendations []Recommendation
}

// ColumnSummary — сводка по колонке.
type ColumnSummary struct {
	ColumnID    valueobject.ColumnID
	Title       string
	Count       int
	Overdue     int
	AvgProgress float64
}

// GoalSummary — сводка по цели.
type GoalSummary struct {
	Goal        string
	TaskCount   int
	AvgProgress float64
}

// Recommendation — рекомендация.
type Recommendation struct {
	Level   string // "info" | "warning" | "critical"
	Message string
}

// NewService создаёт ReviewService.
func NewService(boards boardservice.Service) Service {
	return &service{boards: boards}
}

type service struct {
	boards boardservice.Service
}

// Summarize собирает сводку по доске через snapshot.
func (s *service) Summarize(ctx context.Context, boardID valueobject.BoardID, since *int64) (Summary, error) {
	snap, err := s.boards.GetBoardSnapshot(ctx, boardID, since)
	if err != nil {
		return Summary{}, err
	}

	now := time.Now().UnixMilli()
	sum := Summary{
		BoardID:     boardID,
		BoardTitle:  snap.Board.Title,
		GeneratedAt: now,
	}

	// Группировка по колонкам
	colMap := make(map[valueobject.ColumnID]*ColumnSummary, len(snap.Columns))
	for _, c := range snap.Columns {
		colMap[c.ID] = &ColumnSummary{ColumnID: c.ID, Title: c.Title}
	}

	// Группировка по целям (стикер Client/Goal — условно по title стикера "Client/Goal")
	goalMap := make(map[string]*GoalSummary)
	goalStickerID := findGoalSticker(snap)

	for _, t := range snap.Tasks {
		if t.Deleted || t.Completed {
			continue // не считать удалённые и выполненные в метриках активных
		}
		sum.TotalTasks++
		if t.IsOverdue(now) {
			sum.OverdueCount++
		}

		// по колонке
		if t.ColumnID != nil {
			if cs, ok := colMap[*t.ColumnID]; ok {
				cs.Count++
				if t.IsOverdue(now) {
					cs.Overdue++
				}
			}
		}

		// по цели
		if goalStickerID != nil {
			if v, ok := t.Stickers[*goalStickerID]; ok && v.Value != "" {
				gs, ok := goalMap[v.Value]
				if !ok {
					gs = &GoalSummary{Goal: v.Value}
					goalMap[v.Value] = gs
				}
				gs.TaskCount++
			}
		}
	}

	// Итоги по колонкам
	for _, cs := range colMap {
		if cs.Count > 0 {
			sum.ByColumn = append(sum.ByColumn, *cs)
		}
	}

	// Итоги по целям
	for _, gs := range goalMap {
		sum.ByGoal = append(sum.ByGoal, *gs)
	}

	// Рекомендации
	if sum.OverdueCount > 0 {
		sum.Recommendations = append(sum.Recommendations, Recommendation{
			Level:   "warning",
			Message: "просроченных задач: " + strconv.Itoa(sum.OverdueCount) + " — перенести в Review",
		})
	}
	for _, cs := range sum.ByColumn {
		if cs.Overdue > 0 {
			sum.Recommendations = append(sum.Recommendations, Recommendation{
				Level:   "warning",
				Message: "колонка «" + cs.Title + "»: " + strconv.Itoa(cs.Overdue) + " просроченных",
			})
		}
	}

	return sum, nil
}

// findGoalSticker находит стикер Client/Goal по названию.
func findGoalSticker(snap board.Aggregate) *valueobject.StickerID {
	for _, st := range snap.Stickers {
		if st.Title == "Client/Goal" || st.Title == "Goal" {
			id := st.ID
			return &id
		}
	}
	return nil
}

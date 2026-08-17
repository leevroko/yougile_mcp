// Package goal — доменный сервис отслеживания целей.
package goal

import (
	"context"

	"github.com/yougile-mcp/internal/domain/goal"
	"github.com/yougile-mcp/internal/domain/valueobject"
	boardservice "github.com/yougile-mcp/internal/service/board"
)

// Service — отслеживание прогресса целей (сценарий Goal Tracking).
type Service interface {
	// TrackGoals возвращает агрегат целей доски.
	TrackGoals(ctx context.Context, boardID valueobject.BoardID, since *int64) (goal.Aggregate, error)
	// WeightedKR считает weighted average по стикерам Weight/Progress.
	WeightedKR(ctx context.Context, boardID valueobject.BoardID) ([]GoalProgress, error)
}

// GoalProgress — прогресс одной цели.
type GoalProgress struct {
	Goal        string
	WeightedKR  float64 // сумма(weight×progress) / сумма(weight)
	TotalWeight int
	Tasks       []goal.TaskRef
	Status      string // "on_track" | "at_risk" | "behind"
}

// NewService создаёт GoalService.
func NewService(boards boardservice.Service) Service {
	return &service{boards: boards}
}

type service struct {
	boards boardservice.Service
}

// Пороговые значения статуса.
const (
	statusOnTrack = 0.75
	statusAtRisk  = 0.50
)

// TrackGoals собирает цели доски из snapshot.
func (s *service) TrackGoals(ctx context.Context, boardID valueobject.BoardID, since *int64) (goal.Aggregate, error) {
	snap, err := s.boards.GetBoardSnapshot(ctx, boardID, since)
	if err != nil {
		return goal.Aggregate{}, err
	}

	// Найти стикеры: Client/Goal (string), Weight (number), Progress (number)
	var goalSticker, weightSticker, progressSticker *valueobject.StickerID
	for _, st := range snap.Stickers {
		switch st.Title {
		case "Client/Goal", "Goal":
			goalSticker = &st.ID
		case "Weight":
			weightSticker = &st.ID
		case "Progress":
			progressSticker = &st.ID
		}
	}

	if goalSticker == nil || weightSticker == nil || progressSticker == nil {
		// Не хватает стикеров — вернуть пустой агрегат (нет целей)
		return goal.Aggregate{}, nil
	}

	// Группировка задач по значению Client/Goal
	type keyedTask struct {
		ref      goal.TaskRef
		goalName string
	}
	byGoal := make(map[string][]keyedTask)

	for _, t := range snap.Tasks {
		if t.Deleted || t.Completed {
			continue
		}
		gv, ok := t.Stickers[*goalSticker]
		if !ok || gv.Value == "" {
			continue // задача без цели
		}
		wv, _ := t.Stickers[*weightSticker]
		pv, _ := t.Stickers[*progressSticker]
		ref := goal.TaskRef{
			TaskID:   t.ID,
			Title:    t.Title,
			Weight:   parseInt(wv.Value),
			Progress: parseInt(pv.Value),
		}
		byGoal[gv.Value] = append(byGoal[gv.Value], keyedTask{ref: ref, goalName: gv.Value})
	}

	var agg goal.Aggregate
	for name, kts := range byGoal {
		g := goal.Goal{
			Name:  name,
			Tasks: make([]goal.TaskRef, 0, len(kts)),
		}
		for _, kt := range kts {
			g.Weight += kt.ref.Weight
			g.Progress += kt.ref.Progress
			g.Tasks = append(g.Tasks, kt.ref)
		}
		agg.Goals = append(agg.Goals, g)
	}
	return agg, nil
}

// WeightedKR считает weighted average для каждой цели.
func (s *service) WeightedKR(ctx context.Context, boardID valueobject.BoardID) ([]GoalProgress, error) {
	agg, err := s.TrackGoals(ctx, boardID, nil)
	if err != nil {
		return nil, err
	}

	out := make([]GoalProgress, 0, len(agg.Goals))
	for _, g := range agg.Goals {
		totalWeight := 0
		weightedSum := 0.0
		for _, tr := range g.Tasks {
			totalWeight += tr.Weight
			weightedSum += float64(tr.Weight) * float64(tr.Progress)
		}
		kr := 0.0
		if totalWeight > 0 { // защита от деления на ноль
			kr = weightedSum / float64(totalWeight)
		}
		out = append(out, GoalProgress{
			Goal:        g.Name,
			WeightedKR:  kr,
			TotalWeight: totalWeight,
			Tasks:       g.Tasks,
			Status:      statusFor(kr),
		})
	}
	return out, nil
}

// statusFor определяет статус по прогрессу.
func statusFor(kr float64) string {
	switch {
	case kr >= statusOnTrack:
		return "on_track"
	case kr >= statusAtRisk:
		return "at_risk"
	default:
		return "behind"
	}
}

// parseInt парсит число из строки стикера (number тип).
func parseInt(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	neg := false
	for i, r := range s {
		if i == 0 && r == '-' {
			neg = true
			continue
		}
		if r < '0' || r > '9' {
			return 0 // не число
		}
		n = n*10 + int(r-'0')
	}
	if neg {
		return -n
	}
	return n
}

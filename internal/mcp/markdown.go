package mcp

import (
	"fmt"
	"strings"

	"github.com/yougile-mcp/internal/domain/valueobject"
	auditservice "github.com/yougile-mcp/internal/service/audit"
	reviewservice "github.com/yougile-mcp/internal/service/review"
)

// renderSummary — markdown-сводка (TL;DR сверху).
func renderSummary(s reviewservice.Summary) string {
	var b strings.Builder

	b.WriteString("## " + s.BoardTitle + "\n\n")
	b.WriteString(fmt.Sprintf("**TL;DR**: %d задач, %d просрочено, %d без стикеров\n\n",
		s.TotalTasks, s.OverdueCount, s.MissingSticker))

	if len(s.ByColumn) > 0 {
		b.WriteString("| Колонка | Задачи | Просрочка |\n|---|---|---|\n")
		for _, c := range s.ByColumn {
			b.WriteString(fmt.Sprintf("| %s | %d | %d |\n", c.Title, c.Count, c.Overdue))
		}
		b.WriteString("\n")
	}

	if len(s.ByGoal) > 0 {
		b.WriteString("### По целям\n\n| Цель | Задач |\n|---|---|\n")
		for _, g := range s.ByGoal {
			b.WriteString(fmt.Sprintf("| %s | %d |\n", g.Goal, g.TaskCount))
		}
		b.WriteString("\n")
	}

	if len(s.Recommendations) > 0 {
		b.WriteString("### Рекомендации\n\n")
		for _, r := range s.Recommendations {
			icon := map[string]string{"info": "ℹ️", "warning": "⚠️", "critical": "🚨"}[r.Level]
			b.WriteString(fmt.Sprintf("- %s %s\n", icon, r.Message))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderAudit — markdown-отчёт аудита.
func renderAudit(r auditservice.Result) string {
	var b strings.Builder
	b.WriteString("## Аудит доски\n\n")
	b.WriteString(fmt.Sprintf("**Просроченных**: %d, **без стикеров**: %d, **перемещено в Review**: %d\n\n",
		r.OverdueCount, r.MissingStickerCount, r.AutoMoved))
	if r.DryRun {
		b.WriteString("> режим dry-run: изменения не выполнялись\n\n")
	}
	if len(r.Issues) > 0 {
		b.WriteString("| Тип | Задача | Описание |\n|---|---|---|\n")
		for _, is := range r.Issues {
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", is.Type, is.Title, is.Description))
		}
		b.WriteString("\n")
	} else {
		b.WriteString("Проблем не найдено ✅\n\n")
	}
	return b.String()
}

// renderGoals — markdown-отчёт прогресса целей.
func renderGoals(goals []goalProgressView) string {
	var b strings.Builder
	b.WriteString("## Прогресс целей (weighted KR)\n\n")
	if len(goals) == 0 {
		b.WriteString("Целей не найдено.\n\n")
		return b.String()
	}
	b.WriteString("| Цель | Weighted KR | Статус | Задач |\n|---|---|---|---|\n")
	for _, g := range goals {
		status := map[string]string{"on_track": "🟢", "at_risk": "🟡", "behind": "🔴"}[g.Status]
		b.WriteString(fmt.Sprintf("| %s | %.1f%% | %s %s | %d |\n",
			g.Goal, g.WeightedKR*100, status, g.Status, len(g.Tasks)))
	}
	b.WriteString("\n")
	return b.String()
}

// goalProgressView — отображение прогресса цели.
type goalProgressView struct {
	Goal       string
	WeightedKR float64
	Status     string
	Tasks      []taskRefView
}

type taskRefView struct {
	TaskID   string
	Title    string
	Weight   int
	Progress int
}

// renderStickers — легенда стикеров (вложенные справочные данные).
func renderStickers(stickers map[valueobject.StickerID]string) string {
	if len(stickers) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n**Стикеры**:\n")
	for id, title := range stickers {
		b.WriteString(fmt.Sprintf("- `%s` — %s\n", id.String(), title))
	}
	return b.String()
}

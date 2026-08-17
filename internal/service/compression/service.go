// Package compression — доменный сервис сжатия ревью.
package compression

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	reviewservice "github.com/yougile-mcp/internal/service/review"
)

// Level — уровень сжатия.
type Level string

// Уровни цепочки сжатия.
const (
	LevelDaily   Level = "daily"
	LevelWeekly  Level = "weekly"
	LevelMonthly Level = "monthly"
	LevelYearly  Level = "yearly"
)

// TimeRange — период отчёта.
type TimeRange struct {
	From int64 // ms
	To   int64 // ms
}

// Result — результат сжатия.
type Result struct {
	Level   Level
	Period  TimeRange
	Summary string  // сжатый markdown
	SavedTo *string // путь в memory/reviews/
	Source  *string // путь к исходному отчёту
}

// Service — цепочка сжатия ревью.
type Service interface {
	Compress(ctx context.Context, level Level, period TimeRange, source *string) (Result, error)
}

// NewService создаёт CompressionService.
// memoryDir — путь к memory/reviews/ (из конфигурации).
func NewService(memoryDir string, reviews reviewservice.Service) Service {
	return &service{memoryDir: memoryDir, reviews: reviews}
}

type service struct {
	memoryDir string
	reviews   reviewservice.Service
}

// Compress создаёт сжатый отчёт уровня level за период period.
func (s *service) Compress(ctx context.Context, level Level, period TimeRange, source *string) (Result, error) {
	result := Result{Level: level, Period: period, Source: source}

	// Попытка найти предыдущий отчёт для сжатия
	prevPath := ""
	if source != nil {
		prevPath = *source
	} else {
		prevPath = s.findLatest(ctx, level)
	}

	if prevPath != "" {
		content, err := os.ReadFile(prevPath)
		if err != nil {
			return result, fmt.Errorf("compression: read %s: %w", prevPath, err)
		}
		result.Summary = summarizeLevel(level, string(content))
		result.Source = &prevPath
	} else {
		// Нет предыдущего отчёта — собрать актуальную картину (для daily)
		result.Summary = "Нет предыдущих отчётов для сжатия."
	}

	// Сохранить в memory/reviews/
	if err := os.MkdirAll(s.memoryDir, 0o755); err != nil {
		return result, fmt.Errorf("compression: mkdir: %w", err)
	}
	filename := fmt.Sprintf("%s-%s.md", time.UnixMilli(period.From).Format("2006-01-02"), level)
	full := filepath.Join(s.memoryDir, filename)
	if err := os.WriteFile(full, []byte(result.Summary), 0o644); err != nil {
		return result, fmt.Errorf("compression: write: %w", err)
	}
	result.SavedTo = &full
	return result, nil
}

// findLatest ищет последний отчёт предыдущего уровня в memory/reviews/.
func (s *service) findLatest(ctx context.Context, level Level) string {
	entries, err := os.ReadDir(s.memoryDir)
	if err != nil {
		return ""
	}
	// Ищем по префиксу предыдущего уровня
	prevLevel := previousLevel(level)
	var latest string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.Contains(e.Name(), "-"+string(prevLevel)+".md") {
			latest = filepath.Join(s.memoryDir, e.Name())
		}
	}
	return latest
}

// previousLevel — предыдущий уровень цепочки.
func previousLevel(level Level) Level {
	switch level {
	case LevelWeekly:
		return LevelDaily
	case LevelMonthly:
		return LevelWeekly
	case LevelYearly:
		return LevelMonthly
	default:
		return LevelDaily
	}
}

// summarizeLevel — простая эвристика сжатия: берёт первые строки отчёта.
func summarizeLevel(level Level, content string) string {
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, 20)
	for _, l := range lines {
		if strings.HasPrefix(l, "#") || strings.HasPrefix(l, "-") || strings.HasPrefix(l, "*") {
			kept = append(kept, l)
			if len(kept) >= 15 {
				break
			}
		}
	}
	if len(kept) == 0 {
		return content // нечего сжимать
	}
	header := "# " + strings.ToUpper(string(level)) + " REVIEW\n\n"
	return header + strings.Join(kept, "\n")
}

// Package mcp — MCP-сервер: инструменты двух слоёв + markdown-презентация.
package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/yougile-mcp/internal/domain/valueobject"
)

// format — формат вывода (json | markdown).
type format string

// Форматы вывода.
const (
	formatJSON     format = "json"
	formatMarkdown format = "markdown"
)

// parseFormat нормализует формат из параметра.
func parseFormat(s string) format {
	switch s {
	case "markdown", "md":
		return formatMarkdown
	default:
		return formatJSON
	}
}

// str — безопасное извлечение строки из map.
func str(m map[string]any, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}

// boolVal — извлечение bool из map.
func boolVal(m map[string]any, key string) bool {
	v, ok := m[key].(bool)
	return ok && v
}

// numInt — извлечение числа из map (int или float64).
func numInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

// parseProjectID парсит ProjectID.
func parseProjectID(s string) (valueobject.ProjectID, error) {
	raw, err := valueobject.NewID(s)
	if err != nil {
		return valueobject.ProjectID{}, fmt.Errorf("invalid projectId %q: %w", s, err)
	}
	return valueobject.ProjectID(raw), nil
}

// parseBoardID парсит BoardID.
func parseBoardID(s string) (valueobject.BoardID, error) {
	raw, err := valueobject.NewID(s)
	if err != nil {
		return valueobject.BoardID{}, fmt.Errorf("invalid boardId %q: %w", s, err)
	}
	return valueobject.BoardID(raw), nil
}

// parseColumnID парсит ColumnID.
func parseColumnID(s string) (valueobject.ColumnID, error) {
	raw, err := valueobject.NewID(s)
	if err != nil {
		return valueobject.ColumnID{}, fmt.Errorf("invalid columnId %q: %w", s, err)
	}
	return valueobject.ColumnID(raw), nil
}

// parseTaskID парсит TaskID.
func parseTaskID(s string) (valueobject.TaskID, error) {
	raw, err := valueobject.NewID(s)
	if err != nil {
		return valueobject.TaskID{}, fmt.Errorf("invalid taskId %q: %w", s, err)
	}
	return valueobject.TaskID(raw), nil
}

// parseStickerID парсит StickerID.
func parseStickerID(s string) (valueobject.StickerID, error) {
	raw, err := valueobject.NewID(s)
	if err != nil {
		return valueobject.StickerID{}, fmt.Errorf("invalid stickerId %q: %w", s, err)
	}
	return valueobject.StickerID(raw), nil
}

// toJSON — сериализация в JSON (с отступами).
func toJSON(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

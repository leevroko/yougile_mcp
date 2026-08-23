// Package mcp — MCP-сервер: инструменты двух слоёв + markdown-презентация.
package mcp

import (
	"encoding/json"
	"fmt"
	"strconv"

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

// parseStickers парсит аргумент stickers: {stickerId: значение}.
// Для select-стикеров значение — ID опции, для прочих — строка/число.
// nil, если аргумент не передан.
func parseStickers(m map[string]any, key string) (map[valueobject.StickerID]valueobject.StickerValue, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			return nil, nil
		}
		out := make(map[valueobject.StickerID]valueobject.StickerValue, len(val))
		for k, raw := range val {
			sid, err := parseStickerID(k)
			if err != nil {
				return nil, err
			}
			switch rv := raw.(type) {
			case string:
				out[sid] = valueobject.StickerValue{Value: rv}
			case float64:
				// number-стикеры приходят из JSON как float64
				out[sid] = valueobject.StickerValue{Value: strconv.FormatFloat(rv, 'f', -1, 64)}
			default:
				out[sid] = valueobject.StickerValue{Value: fmt.Sprintf("%v", raw)}
			}
		}
		return out, nil
	case string:
		// строка-JSON для клиентов, не умеющих вложенные объекты
		var parsed map[string]any
		if err := json.Unmarshal([]byte(val), &parsed); err != nil {
			return nil, fmt.Errorf("invalid stickers: ожидается объект {stickerId: значение}: %w", err)
		}
		return parseStickers(map[string]any{key: parsed}, key)
	default:
		return nil, fmt.Errorf("invalid stickers: ожидается объект {stickerId: значение}")
	}
}

// toJSON — сериализация в JSON (с отступами).
func toJSON(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

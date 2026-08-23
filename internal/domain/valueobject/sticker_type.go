package valueobject

import "encoding/json"

// StickerType — тип кастомного стикера (старый /stickers).
type StickerType string

// Типы кастомных стикеров.
const (
	StickerTypeString StickerType = "string"
	StickerTypeSelect StickerType = "select"
	StickerTypeNumber StickerType = "number"
	StickerTypeDate   StickerType = "date"
	StickerTypeUser   StickerType = "user"
)

// StickerValue — значение стикера в задаче.
// Для select — ID опции (StateID); для string/number/date — текст/число.
type StickerValue struct {
	Kind  StickerType // тип стикера
	Value string      // значение
}

// MarshalJSON сериализует значение стикера как строку:
// {"<stickerId>": "<value>"} — без обёртки Kind/Value.
func (v StickerValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Value)
}

// UnmarshalJSON читает значение стикера из строки (или объекта с полем value).
func (v *StickerValue) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		v.Value = s
		return nil
	}
	type alias StickerValue
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*v = StickerValue(a)
	return nil
}

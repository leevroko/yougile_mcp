package valueobject

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

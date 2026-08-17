// Package valueobject содержит value objects домена: идентификаторы,
// пагинацию, дедлайны, цвета, роли, типы стикеров.
package valueobject

import (
	"errors"
	"fmt"
	"regexp"
)

// ErrInvalidID — некорректный идентификатор (не UUID).
var ErrInvalidID = errors.New("invalid id: must be a UUID")

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ID — базовый идентификатор (UUID). Валидация в конструкторе.
type ID struct {
	value string
}

// NewID создаёт ID из строки, проверяя UUID-формат.
// Пустая строка возвращает ErrInvalidID.
func NewID(s string) (ID, error) {
	if !uuidRe.MatchString(s) {
		return ID{}, fmt.Errorf("%w: %q", ErrInvalidID, s)
	}
	return ID{value: s}, nil
}

// MustID создаёт ID без проверки; паникует при некорректном значении.
// Используется только для констант и тестов.
func MustID(s string) ID {
	id, err := NewID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// String возвращает строковое представление ID.
func (id ID) String() string { return id.value }

// IsZero возвращает true, если ID пустой (нулевое значение).
func (id ID) IsZero() bool { return id.value == "" }

// Типизированные ID — обёртки для type-safety на уровне API.
type ProjectID ID

// String возвращает строковое представление.
func (id ProjectID) String() string { return ID(id).String() }

// IsZero возвращает true для пустого ID.
func (id ProjectID) IsZero() bool { return ID(id).IsZero() }

// NewProjectID создаёт ProjectID с валидацией UUID.
func NewProjectID(s string) (ProjectID, error) {
	id, err := NewID(s)
	if err != nil {
		return ProjectID{}, err
	}
	return ProjectID(id), nil
}

type BoardID ID

func (id BoardID) String() string { return ID(id).String() }
func (id BoardID) IsZero() bool   { return ID(id).IsZero() }
func NewBoardID(s string) (BoardID, error) {
	id, err := NewID(s)
	if err != nil {
		return BoardID{}, err
	}
	return BoardID(id), nil
}

type ColumnID ID

func (id ColumnID) String() string { return ID(id).String() }
func (id ColumnID) IsZero() bool   { return ID(id).IsZero() }
func NewColumnID(s string) (ColumnID, error) {
	id, err := NewID(s)
	if err != nil {
		return ColumnID{}, err
	}
	return ColumnID(id), nil
}

type TaskID ID

func (id TaskID) String() string { return ID(id).String() }
func (id TaskID) IsZero() bool   { return ID(id).IsZero() }
func NewTaskID(s string) (TaskID, error) {
	id, err := NewID(s)
	if err != nil {
		return TaskID{}, err
	}
	return TaskID(id), nil
}

type UserID ID

func (id UserID) String() string { return ID(id).String() }
func (id UserID) IsZero() bool   { return ID(id).IsZero() }
func NewUserID(s string) (UserID, error) {
	id, err := NewID(s)
	if err != nil {
		return UserID{}, err
	}
	return UserID(id), nil
}

type StickerID ID

func (id StickerID) String() string { return ID(id).String() }
func (id StickerID) IsZero() bool   { return ID(id).IsZero() }
func NewStickerID(s string) (StickerID, error) {
	id, err := NewID(s)
	if err != nil {
		return StickerID{}, err
	}
	return StickerID(id), nil
}

type StringStickerID ID

func (id StringStickerID) String() string { return ID(id).String() }
func (id StringStickerID) IsZero() bool   { return ID(id).IsZero() }
func NewStringStickerID(s string) (StringStickerID, error) {
	id, err := NewID(s)
	if err != nil {
		return StringStickerID{}, err
	}
	return StringStickerID(id), nil
}

type SprintStickerID ID

func (id SprintStickerID) String() string { return ID(id).String() }
func (id SprintStickerID) IsZero() bool   { return ID(id).IsZero() }
func NewSprintStickerID(s string) (SprintStickerID, error) {
	id, err := NewID(s)
	if err != nil {
		return SprintStickerID{}, err
	}
	return SprintStickerID(id), nil
}

// StateID — ID состояния стикера (опция select, состояние string/sprint).
type StateID ID

func (id StateID) String() string { return ID(id).String() }
func (id StateID) IsZero() bool   { return ID(id).IsZero() }
func NewStateID(s string) (StateID, error) {
	id, err := NewID(s)
	if err != nil {
		return StateID{}, err
	}
	return StateID(id), nil
}

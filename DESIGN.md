# DESIGN: DDD-модель YouGile MCP-сервера

> Результат шага 3 (PLAN.md): дизайн DDD-сущностей на основе пользовательских сценариев (SCENARIOS.md).
> Проект в фазе проектирования — это дизайн-документ, Go-код будет написан позже по этому дизайну.
> Согласованные принципы: гибрид двух слоёв инструментов, вложенные справочные данные, `format: json|markdown`, полная пагинация, дельта `since`, без TTL-кэша.

---

## 1. Общие принципы модели

- **Domain-пакеты самодостаточны**: entity, value objects, repository interface в одном пакете. Никакого общего `models.go`.
- **Интерфейсы в domain**, реализации в infrastructure (repository). Сервисы зависят только от интерфейсов.
- **Модель выстроена под сценарии** (SCENARIOS.md), не наоборот:
  - Goal Tracking требует **Goal-агрегат** с weighted KR (стикеры Weight/Progress)
  - Audit/Summarize требует **deadline** как first-class value object (просрочка в ms)
  - Bulk move / Batch stickers требуют **пагинацию полных данных** (PagingMetadata)
  - Quick Actions требует **частичное обновление** (update только переданных полей)
- **Никакой магии**: только то, что реально нужно сценариям. Без TTL-кэша, без event bus в модели (они в service layer).

---

## 2. Value Objects

```go
// internal/domain/valueobject/id.go
package valueobject

// Пакет valueobject: идентификаторы и примитивные типы с валидацией.
// Валидация — в конструкторах; инварианты защищены типами.

// ID — базовый идентификатор (UUID). Все ID наследуют валидацию.
type ID struct { value string }

func NewID(s string) (ID, error)          // валидация UUID
func (id ID) String() string              // значение
func (id ID) IsZero() bool                // пустой ID

// Типизированные ID (обёртки над ID, разные типы для type-safety)
type ProjectID ID
type BoardID ID
type ColumnID ID
type TaskID ID
type UserID ID
type StickerID ID
type StringStickerID ID
type SprintStickerID ID
type StateID ID
```

```go
// internal/domain/valueobject/paging.go
package valueobject

// PagingMetadata — пагинация (совпадает с API).
type PagingMetadata struct {
    Count  int  // элементов в ответе
    Limit  int  // максимум на страницу
    Offset int  // индекс первого элемента
    Next   bool // есть ли ещё страницы
}

func (p PagingMetadata) HasNext() bool { return p.Next }
```

```go
// internal/domain/valueobject/deadline.go
package valueobject

// Deadline — дедлайн задачи (timestamp в миллисекундах).
// Используется сценариями Audit/Summarize для расчёта просрочки.
type Deadline struct {
    value int64 // timestamp ms
}

func NewDeadline(ms int64) Deadline         // валидация ms > 0
func (d Deadline) IsOverdue(nowMS int64) bool // d.value < nowMS
func (d Deadline) OverdueDays(nowMS int64) int
func (d Deadline) IsZero() bool
func (d Deadline) Value() int64
```

```go
// internal/domain/valueobject/color.go
package valueobject

// Color — цвет колонки (числовой код) или задачи (строковый класс).
type ColumnColor int   // 1 = #7B869E (по API)
type TaskColor string  // "task-primary", "task-gray", ...

const (
    ColumnColorDefault ColumnColor = 1
    // ... другие по API
)

const (
    TaskColorPrimary TaskColor = "task-primary"
    TaskColorGray    TaskColor = "task-gray"
    // ...
)
```

```go
// internal/domain/valueobject/role.go
package valueobject

// Роли пользователей (по API: worker / admin / observer / customer).
type Role string

const (
    RoleWorker   Role = "worker"
    RoleAdmin    Role = "admin"
    RoleObserver Role = "observer"
    RoleCustomer Role = "customer"
)
```

```go
// internal/domain/valueobject/sticker_type.go
package valueobject

// Типы кастомных стикеров (старый /stickers).
type StickerType string

const (
    StickerTypeString StickerType = "string"
    StickerTypeSelect StickerType = "select"
    StickerTypeNumber StickerType = "number"
    StickerTypeDate   StickerType = "date"
    StickerTypeUser   StickerType = "user"
)

// Значение стикера в задаче: string | number | StickerID (select)
type StickerValue struct {
    Kind  StickerType // тип стикера
    Value string      // для select — ID опции; для string/number — текст/число
}
```

---

## 3. Entities

```go
// internal/domain/project/project.go
package project

import "github.com/yougile-mcp/internal/domain/valueobject"

type Project struct {
    ID        valueobject.ProjectID
    Title     string
    Timestamp int64
    Users     map[valueobject.UserID]valueobject.Role // userId: role
    Deleted   bool
}

// Repository
type Repository interface {
    List(ctx context.Context) ([]Project, valueobject.PagingMetadata, error)
    GetByID(ctx context.Context, id valueobject.ProjectID) (Project, error)
    Create(ctx context.Context, req CreateRequest) (valueobject.ProjectID, error)
    Update(ctx context.Context, id valueobject.ProjectID, req UpdateRequest) error
}

type CreateRequest struct {
    Title string
    Users map[valueobject.UserID]valueobject.Role
}

type UpdateRequest struct {
    Title   *string
    Users   *map[valueobject.UserID]valueobject.Role
    Deleted *bool
}
```

```go
// internal/domain/board/board.go
package board

import "github.com/yougile-mcp/internal/domain/valueobject"

type Board struct {
    ID        valueobject.BoardID
    Title     string
    ProjectID valueobject.ProjectID
    Stickers  []BoardStickerSetting // настройки стикеров доски
    Deleted   bool
}

// BoardStickerSetting — настройка одного кастомного стикера доски
type BoardStickerSetting struct {
    StickerID valueobject.StickerID
    Title     string
    Type      valueobject.StickerType
    Options   []StickerOption // для select: {id, title, color}
}

type StickerOption struct {
    ID    valueobject.StateID
    Title string
    Color *string // hex
}

type Repository interface {
    List(ctx context.Context, projectID valueobject.ProjectID) ([]Board, valueobject.PagingMetadata, error)
    GetByID(ctx context.Context, id valueobject.BoardID) (Board, error)
    Create(ctx context.Context, req CreateRequest) (valueobject.BoardID, error)
    Update(ctx context.Context, id valueobject.BoardID, req UpdateRequest) error
}

type CreateRequest struct {
    Title     string
    ProjectID valueobject.ProjectID
    Stickers  []CreateStickerSetting
}

type UpdateRequest struct {
    Title     *string
    ProjectID *valueobject.ProjectID
    Deleted   *bool
}
```

```go
// internal/domain/column/column.go
package column

import "github.com/yougile-mcp/internal/domain/valueobject"

type Column struct {
    ID      valueobject.ColumnID
    Title   string
    BoardID valueobject.BoardID
    Color   valueobject.ColumnColor
    Deleted bool
}

type Repository interface {
    List(ctx context.Context, boardID valueobject.BoardID) ([]Column, valueobject.PagingMetadata, error)
    GetByID(ctx context.Context, id valueobject.ColumnID) (Column, error)
    Create(ctx context.Context, req CreateRequest) (valueobject.ColumnID, error)
    Update(ctx context.Context, id valueobject.ColumnID, req UpdateRequest) error
}

type CreateRequest struct {
    Title   string
    BoardID valueobject.BoardID
    Color   valueobject.ColumnColor
}

type UpdateRequest struct {
    Title   *string
    BoardID *valueobject.BoardID
    Color   *valueobject.ColumnColor
    Deleted *bool
}
```

```go
// internal/domain/task/task.go
package task

import "github.com/yougile-mcp/internal/domain/valueobject"

// Task — основная сущность. Задействована во всех сценариях.
type Task struct {
    ID                valueobject.TaskID
    Title             string
    Timestamp         int64
    ColumnID          *valueobject.ColumnID // nil = без колонки
    Description       string
    Completed         bool
    CompletedTimestamp *int64
    Archived          bool
    ArchivedTimestamp *int64
    Deleted           bool
    Assigned          []valueobject.UserID
    CreatedBy         *valueobject.UserID
    Stickers          map[valueobject.StickerID]valueobject.StickerValue // {stickerId: value}
    Deadline          *valueobject.Deadline
    Subtasks          []valueobject.TaskID
    Color             valueobject.TaskColor
    // checklists, stopwatch, timer, timeTracking — по мере необходимости сценариев
}

// Методы для сценариев
func (t Task) IsOverdue(nowMS int64) bool {
    return t.Deadline != nil && t.Deadline.IsOverdue(nowMS)
}

func (t Task) StickerValue(id valueobject.StickerID) (valueobject.StickerValue, bool) {
    v, ok := t.Stickers[id]
    return v, ok
}

type Repository interface {
    // List — с фильтрацией по колонке/доске; полная пагинация в сервисе.
    List(ctx context.Context, filter Filter) ([]Task, valueobject.PagingMetadata, error)
    GetByID(ctx context.Context, id valueobject.TaskID) (Task, error)
    Create(ctx context.Context, req CreateRequest) (valueobject.TaskID, error)
    Update(ctx context.Context, id valueobject.TaskID, req UpdateRequest) error
    // Delete — мягкое удаление (deleted=true), как в API
    Delete(ctx context.Context, id valueobject.TaskID) error
}

type Filter struct {
    BoardID  *valueobject.BoardID   // без columnId — все задачи доски (неопределённость №6)
    ColumnID *valueobject.ColumnID  // конкретная колонка
    Limit    int
    Offset   int
}

type CreateRequest struct {
    Title       string
    ColumnID    *valueobject.ColumnID
    Description string
    Deadline    *valueobject.Deadline
    Completed   bool
    Assigned    []valueobject.UserID
    Stickers    map[valueobject.StickerID]valueobject.StickerValue
    Subtasks    []valueobject.TaskID
    Color       valueobject.TaskColor
}

// UpdateRequest — частичное обновление: только переданные поля (Quick Actions)
type UpdateRequest struct {
    Title       *string
    ColumnID    *valueobject.ColumnID // "-" → nil = снять с колонки
    Description *string
    Deadline    *valueobject.Deadline
    Completed   *bool
    Archived    *bool
    Deleted     *bool
    Assigned    *[]valueobject.UserID
    Stickers    *map[valueobject.StickerID]valueobject.StickerValue
    Subtasks    *[]valueobject.TaskID
    Color       *valueobject.TaskColor
}
```

```go
// internal/domain/sticker/sticker.go
package sticker

import "github.com/yougile-mcp/internal/domain/valueobject"

// Sticker — кастомный стикер доски (старый /stickers).
type Sticker struct {
    ID      valueobject.StickerID
    Title   string
    Type    valueobject.StickerType
    BoardID valueobject.BoardID
    Options []StickerOption // select
}

type StickerOption struct {
    ID    valueobject.StateID
    Title string
    Color *string
}

type Repository interface {
    List(ctx context.Context, boardID valueobject.BoardID) ([]Sticker, error)
    GetByID(ctx context.Context, id valueobject.StickerID) (Sticker, error)
    Create(ctx context.Context, req CreateRequest) (valueobject.StickerID, error)
    Update(ctx context.Context, id valueobject.StickerID, req UpdateRequest) error
}
```

```go
// internal/domain/sticker/string_sticker.go
package sticker

// StringSticker — новый механизм (dropdown с состояниями).
type StringSticker struct {
    ID     valueobject.StringStickerID
    Name   string
    Icon   string
    States []StringStickerState
    Deleted bool
}

type StringStickerState struct {
    ID    valueobject.StateID
    Name  string
    Color *string
    Deleted bool
}

// SprintSticker — спринт-стикер (с датами begin/end в СЕКУНДАХ).
type SprintSticker struct {
    ID     valueobject.SprintStickerID
    Name   string
    States []SprintStickerState
    Deleted bool
}

type SprintStickerState struct {
    ID    valueobject.StateID
    Name  string
    Begin int64 // секунды, НЕ ms (отличие от deadline!)
    End   int64
    Deleted bool
}
```

```go
// internal/domain/user/user.go
package user

import "github.com/yougile-mcp/internal/domain/valueobject"

type User struct {
    ID           valueobject.UserID
    Email        string
    RealName     string
    Status       string // online / offline
    LastActivity int64
    IsAdmin      bool
}

type Repository interface {
    List(ctx context.Context) ([]User, valueobject.PagingMetadata, error)
    GetByID(ctx context.Context, id valueobject.UserID) (User, error)
    GetMe(ctx context.Context) (User, error)
    Create(ctx context.Context, req CreateRequest) (valueobject.UserID, error)
    Update(ctx context.Context, id valueobject.UserID, req UpdateRequest) error
}
```

---

## 4. Aggregates

```go
// internal/domain/board/aggregate.go
package board

// BoardAggregate — доска с колонками и задачами.
// Используется сценариями Summarize/Audit/Goal Tracking (snapshot).
type Aggregate struct {
    Board    Board
    Columns  []Column
    Tasks    []Task
    Stickers []Sticker // легенда для расшифровки
}
```

```go
// internal/domain/task/aggregate.go
package task

// TaskAggregate — задача с подзадачами.
type Aggregate struct {
    Task     Task
    Subtasks []Task // подзадачи (по Subtasks IDs)
}
```

```go
// internal/domain/goal/goal.go
package goal

import "github.com/yougile-mcp/internal/domain/valueobject"

// Goal — цель с ключевыми результатами (KR).
// Сценарий Goal Tracking: weighted average KR.
type Goal struct {
    // Стикеры цели (по API — кастомные стикеры доски):
    // Client/Goal (string) — название цели
    // Weight (number) — вес KR
    // Progress (number) — прогресс 0-100
    Name    string // значение стикера Client/Goal
    Weight  int    // значение стикера Weight
    Progress int   // значение стикера Progress (0-100)

    Tasks []TaskRef // задачи, относящиеся к этой цели
}

// TaskRef — ссылка на задачу с её вкладом в цель.
type TaskRef struct {
    TaskID   valueobject.TaskID
    Title    string
    Weight   int
    Progress int
}

// Aggregate: все цели доски + их KR.
type Aggregate struct {
    Goals []Goal
}
```

---

## 5. Repository Interfaces (сводно)

| Пакет | Интерфейс | Методы | Для сценариев |
|-------|-----------|--------|---------------|
| `project` | `Repository` | List, GetByID, Create, Update | Quick Actions (create), list |
| `board` | `Repository` | List, GetByID, Create, Update | Quick Actions, list |
| `column` | `Repository` | List, GetByID, Create, Update | Summarize/Audit (колонки) |
| `task` | `Repository` | List, GetByID, Create, Update, Delete | **все** |
| `sticker` | `Repository` | List, GetByID, Create, Update | Summarize/Audit (расшифровка) |
| `user` | `Repository` | List, GetByID, GetMe, Create, Update | snapshot (пользователи) |

**Маппинг HTTP-статусов** (в infrastructure):
- `404 → ErrNotFound`
- `429 → ErrRateLimited`
- `401 → ErrUnauthorized`
- `400 → ErrBadRequest`
- `5xx → ErrServerError`

---

## 6. Отражение сценариев в модели

| Сценарий | Ключевые сущности | Особенности |
|----------|------------------|-------------|
| Quick Actions | Task, Project, Board, Column | частичное Update (только переданные поля) |
| Bulk Move | Task, Column | Filter по колонке, полная пагинация |
| Batch Stickers | Task, Sticker | валидация легенды до операции |
| Summarize | Board, Column, Task, Sticker | BoardAggregate (snapshot) |
| Audit | Task (deadline), Column (Review), Sticker | IsOverdue, поиск колонки Review |
| Goal Tracking | Goal (aggregate), Task, Sticker | weighted KR, защита от деления на 0 |
| Compression | (файлы отчётов) | не требует новых сущностей |

---

## 7. Что НЕ входит в модель (намеренно)

- **TTL-кэш** — принцип из шага 2: не добавлять на старте
- **Event Bus** — в service layer (шаг 4), не в domain
- **Чеклисты/секундомеры/таймеры** — в Task как опциональные поля, но без логики (сценарии не требуют)
- **Webhooks, чаты, файлы** — исключены из плана (не нужны сценариям)

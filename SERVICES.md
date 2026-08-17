# SERVICES: Дизайн доменных сервисов

> Результат шага 4 (PLAN.md): дизайн доменных сервисов на основе сценариев (SCENARIOS.md) и DDD-модели (DESIGN.md).
> **Решение**: вместо Event Bus — прямые вызовы между сервисами (принцип Simplicity First, без спекулятивной асинхронности).

---

## 1. Принципы

- **Сервисы зависят от репозиториев (интерфейсы из domain), не друг от друга жёстко.**
- **Прямые вызовы вместо событий**: если сервису A нужно действие сервиса B — он вызывает его метод напрямую. Никакого pub/sub.
- **Сервис возвращает структурированные данные** (DTO/сущности). Markdown-форматирование — отдельный презентационный слой (шаг 5).
- **Композитные сценарии инкапсулируют полный обход** (пагинация всех страниц).
- **Параметр `since`** — дельта-обновления (для Summarize/Goal).
- **`dryRun`** — режим без мутаций (Audit, Bulk move, Batch stickers).

---

## 2. BoardService

```go
// internal/service/board/board.go
package board

// BoardService — CRUD досок/колонок/стикеров + snapshot.
type Service interface {
    ListProjects(ctx context.Context) ([]project.Project, valueobject.PagingMetadata, error)
    ListBoards(ctx context.Context, projectID valueobject.ProjectID) ([]board.Board, valueobject.PagingMetadata, error)
    ListColumns(ctx context.Context, boardID valueobject.BoardID) ([]column.Column, error)
    ListStickers(ctx context.Context, boardID valueobject.BoardID) ([]sticker.Sticker, error)

    // GetBoardSnapshot — полное состояние доски (сценарий snapshot).
    // Обходит все колонки и все страницы задач. Для Summarize/Goal.
    GetBoardSnapshot(ctx context.Context, boardID valueobject.BoardID, since *int64) (board.Aggregate, error)

    // CreateProject / CreateBoard / CreateColumn — быстрые операции
    CreateProject(ctx context.Context, title string) (valueobject.ProjectID, error)
    CreateBoard(ctx context.Context, title string, projectID valueobject.ProjectID) (valueobject.BoardID, error)
    CreateColumn(ctx context.Context, title string, boardID valueobject.BoardID, color valueobject.ColumnColor) (valueobject.ColumnID, error)
}

// boardService — реализация
type boardService struct {
    projects project.Repository
    boards   board.Repository
    columns  column.Repository
    stickers sticker.Repository
    tasks    task.Repository // для snapshot
}
```

**Логика**:
- `GetBoardSnapshot`: грузит доску → колонки → для каждой колонки все задачи (полный обход пагинации) → легенда стикеров. Если `since` — фильтровать задачи по изменению (по API нет фильтра по времени, фильтруем на клиенте).
- CRUD — тонкая обёртка над репозиториями.

---

## 3. TaskService

```go
// internal/service/task/task.go
package task

// TaskService — CRUD задач + массовые операции.
type Service interface {
    // Quick Actions (сценарий 5)
    CreateTask(ctx context.Context, req CreateTaskParams) (valueobject.TaskID, error)
    GetTask(ctx context.Context, taskID valueobject.TaskID) (task.Task, error)
    UpdateTask(ctx context.Context, taskID valueobject.TaskID, req task.UpdateRequest) error
    MoveTask(ctx context.Context, taskID valueobject.TaskID, columnID valueobject.ColumnID) error
    DeleteTask(ctx context.Context, taskID valueobject.TaskID) error

    // ListTasks — с вложенными справочными данными (колонки + легенда стикеров)
    ListTasks(ctx context.Context, boardID valueobject.BoardID, columnID *valueobject.ColumnID) (ListResult, error)

    // Bulk Move (сценарий 1)
    BulkMove(ctx context.Context, req BulkMoveParams) (BulkMoveResult, error)

    // Batch Stickers (сценарий 6)
    BatchUpdateStickers(ctx context.Context, req BatchStickersParams) (BatchStickersResult, error)
}

type CreateTaskParams struct {
    Title       string
    ColumnID    *valueobject.ColumnID
    Description string
    Deadline    *valueobject.Deadline
    Assigned    []valueobject.UserID
    Stickers    map[valueobject.StickerID]valueobject.StickerValue
}

// ListResult — задачи + справочные данные (для вложения в ответ)
type ListResult struct {
    Tasks      []task.Task
    Columns    []column.Column     // названия колонок
    StickerMap map[valueobject.StickerID]sticker.Sticker // легенда
}

type BulkMoveParams struct {
    BoardID          valueobject.BoardID
    SourceColumnID   *valueobject.ColumnID // источник: колонка ИЛИ
    TaskIDs          []valueobject.TaskID  // конкретные задачи
    TargetColumnID   valueobject.ColumnID
    Filter           *TaskFilter // по стикерам/статусу
    DryRun           bool
}

type BulkMoveResult struct {
    Moved    int
    Failed   int
    NotFound int
    Details  []MoveDetail // {taskID, status}
}

type BatchStickersParams struct {
    BoardID  valueobject.BoardID
    TaskIDs  []valueobject.TaskID
    Stickers map[valueobject.StickerID]valueobject.StickerValue
    DryRun   bool
}

type BatchStickersResult struct {
    Updated  int
    Failed   int
    NotFound int
}
```

**Логика**:
- `ListTasks`: репозиторий task → справочные колонки/стикеры (1 доп. запрос) → вложить в результат.
- `BulkMove`: получить задачи источника → отфильтровать → для каждой PUT с `{columnId: target}` (только изменяемое поле). Идемпотентно: задача уже в целевой → пропустить. Сериализация N×PUT (семафор, rate limit).
- `BatchUpdateStickers`: валидация легенды стикеров **до** операции → для каждой PUT с `{stickers}`.
- `dryRun`: только чтение, без PUT.

---

## 4. ReviewService

```go
// internal/service/review/review.go
package review

// ReviewService — аналитика доски (сценарий Summarize).
type Service interface {
    // Summarize — сводка: метрики, группировка, рекомендации.
    Summarize(ctx context.Context, boardID valueobject.BoardID, since *int64) (Summary, error)
}

type Summary struct {
    BoardID        valueobject.BoardID
    BoardTitle     string
    GeneratedAt    int64 // timestamp ms
    TotalTasks     int
    OverdueCount   int
    MissingSticker int
    AvgProgress    float64
    ByColumn       []ColumnSummary  // {columnID, title, count, overdue, avgProgress}
    ByGoal         []GoalSummary    // {goal, tasks, avgProgress}
    Recommendations []Recommendation // {level, message}
}

type ColumnSummary struct {
    ColumnID   valueobject.ColumnID
    Title      string
    Count      int
    Overdue    int
    AvgProgress float64
}

type GoalSummary struct {
    Goal        string
    TaskCount   int
    AvgProgress float64
}

type Recommendation struct {
    Level   string // "info" | "warning" | "critical"
    Message string
}
```

**Логика**:
- Вызвать `BoardService.GetBoardSnapshot` → сгруппировать по колонкам, посчитать метрики.
- Рекомендации по правилам: просроченные → «перенести в Review»; задачи без стикеров → «заполнить стикеры»; колонка перегружена (>WIP) → «разгрузить».
- `since` — агрегировать только изменённые задачи (дельта).

---

## 5. AuditService

```go
// internal/service/audit/audit.go
package audit

// AuditService — аудит доски (сценарий Audit).
type Service interface {
    Audit(ctx context.Context, req AuditParams) (AuditResult, error)
}

type AuditParams struct {
    BoardID  valueobject.BoardID
    Rules    Rules
    DryRun   bool
}

type Rules struct {
    Overdue          bool // просрочка
    MissingStickers  bool // нет обязательных стикеров
    AutoMove         bool // перемещать просроченные в Review
}

type AuditResult struct {
    Issues             []Issue
    OverdueCount       int
    MissingStickerCount int
    AutoMoved          int
    DryRun             bool
}

type Issue struct {
    Type        string // "overdue" | "missing_sticker"
    TaskID      valueobject.TaskID
    Title       string
    Description string
}
```

**Логика**:
- `BoardService.GetBoardSnapshot` → для каждой задачи проверить правила:
  - `overdue`: `task.IsOverdue(now)` (deadline в ms).
  - `missing_stickers`: задача без обязательных стикеров (список обязательных — конфиг или стикеры Weight/Progress).
- `autoMove`: найти колонку Review (по названию) → переместить просроченные (через `TaskService.MoveTask`). Если колонки нет — не перемещать, отметить в отчёте.
- `dryRun`: только сбор Issues, без перемещений.

---

## 6. GoalService

```go
// internal/service/goal/goal.go
package goal

// GoalService — отслеживание прогресса целей (сценарий Goal Tracking).
type Service interface {
    TrackGoals(ctx context.Context, boardID valueobject.BoardID, since *int64) (goal.Aggregate, error)
    // WeightedKR — расчёт weighted average по стикерам Weight/Progress.
    WeightedKR(ctx context.Context, boardID valueobject.BoardID) ([]GoalProgress, error)
}

type GoalProgress struct {
    Goal         string   // значение стикера Client/Goal
    WeightedKR   float64  // сумма(weight×progress) / сумма(weight)
    TotalWeight  int
    Tasks        []goal.TaskRef
    Status       string   // "on_track" | "at_risk" | "behind"
}
```

**Логика**:
- `BoardService.GetBoardSnapshot` → собрать задачи по стикеру Client/Goal → сгруппировать по значению.
- Для каждой группы: `weightedKR = Σ(weight × progress) / Σ(weight)`.
- **Защита от деления на ноль**: `Σ(weight) == 0` → пропустить (или `WeightedKR = 0`).
- **Без двойного счёта**: уникальные taskIds (задача, принадлежащая нескольким целям, считается в каждой цели, но не суммируется повторно в общем).
- `Status` по порогам: `≥0.75 on_track`, `0.5–0.75 at_risk`, `<0.5 behind` (пороги конфигурируемые).

---

## 7. CompressionService

```go
// internal/service/compression/compression.go
package compression

// CompressionService — цепочка сжатия ревью (сценарий Compression).
type Service interface {
    // Compress — daily → weekly → monthly → yearly.
    Compress(ctx context.Context, level Level, period TimeRange, source *string) (Result, error)
}

type Level string

const (
    LevelDaily   Level = "daily"
    LevelWeekly  Level = "weekly"
    LevelMonthly Level = "monthly"
    LevelYearly  Level = "yearly"
)

type TimeRange struct {
    From int64 // ms
    To   int64 // ms
}

type Result struct {
    Level    Level
    Period   TimeRange
    Summary  string   // сжатый markdown
    SavedTo  *string  // путь в memory/reviews/
    Source   *string  // путь к исходному отчёту
}
```

**Логика**:
- Читает предыдущие отчёты из файловой системы `memory/reviews/YYYY-MM-DD-{level}.md` (как легаси).
- Сжимает: daily → weekly (воскресенье), weekly → monthly (последний день), monthly → yearly (31 дек).
- Если предыдущего отчёта нет — создать с нуля (собрать актуальную картину через ReviewService).
- Сохраняет в `memory/reviews/`, возвращает путь.

---

## 8. Отражение сценариев в сервисах

| Сценарий | Сервис | Метод | Режимы |
|----------|--------|-------|--------|
| Quick Actions | TaskService | CreateTask/UpdateTask/MoveTask/GetTask/DeleteTask | — |
| List tasks | TaskService | ListTasks (с вложенными справочными) | — |
| Bulk Move | TaskService | BulkMove | dryRun |
| Batch Stickers | TaskService | BatchUpdateStickers | dryRun |
| Summarize | ReviewService | Summarize | since |
| Audit | AuditService | Audit | rules, dryRun, autoMove |
| Goal Tracking | GoalService | TrackGoals/WeightedKR | since |
| Compression | CompressionService | Compress | — |
| Snapshot | BoardService | GetBoardSnapshot | since |

---

## 9. Прямые вызовы между сервисами (вместо Event Bus)

| Откуда | Куда | Метод | Когда |
|--------|------|-------|-------|
| TaskService.CreateTask | AuditService | (проверка стикеров опциональна) | после создания |
| ReviewService | BoardService | GetBoardSnapshot | summarize |
| AuditService | BoardService | GetBoardSnapshot | audit |
| AuditService | TaskService | MoveTask | autoMove |
| GoalService | BoardService | GetBoardSnapshot | track goals |
| CompressionService | ReviewService | Summarize | нет предыдущего отчёта |

> **Важно**: связи направлены от «потребителя» к «провайдеру» (сервис, владеющий данными). Никаких циклических зависимостей. Если связь станет двунаправленной — вынести общий код в низкоуровневый сервис.

---

## 10. Что НЕ входит (сознательно)

- **Event Bus / pub-sub** — заменён прямыми вызовами (решение пользователя, принцип Simplicity First)
- **TTL-кэш** — не добавляем на старте
- **Маркдаун-рендеринг** — презентационный слой, в шаге 5 (MCP layer)
- **Конфигурация обязательных стикеров** — простая константа/конфиг, не отдельная сущность

# SCENARIOS: Пользовательские сценарии ИИ-ассистента

> Документ спецификаций сценариев работы ИИ-ассистента (OpenClaw/pi) с YouGile через MCP-сервер.
> Основа для дизайна DDD-сущностей (PLAN.md шаг 3).
> Согласованные принципы (из PLAN.md шаг 2): гибрид двух слоёв инструментов, вложенные справочные данные, `format: json|markdown`, полная пагинация, дельта `since`, без TTL-кэша на старте.

---

## 1. Bulk Move — массовое перемещение задач

**Цель**: переместить группу задач между колонками (например, все просроченные → Review).

**Входы**:
- `boardId` (uuid) — доска
- `sourceColumnId` / `taskIds` — источник: колонка ИЛИ конкретные задачи
- `targetColumnId` (uuid) — колонка назначения
- `filter` (опц.) — фильтр по стикерам/статусу (просроченные, без стикеров, прогресс)
- `dryRun` (опц., bool) — показать, что будет перемещено, без изменений

**Выходы**:
- `moved` (int) — сколько перемещено
- `failed` (int) — сколько не удалось
- `notFound` (int) — сколько не найдено
- `summary` (markdown) — детализация по задачам

**API-эндпоинты**:
- `GET /tasks?boardId=&columnId=` — получить задачи источника
- `PUT /tasks/{id}` с `{columnId: target}` — переместить каждую

**Граничные случаи**:
- Пустой источник → 0 перемещено, без ошибок
- Задача уже в целевой колонке → пропустить (idempotent)
- Пагинация: источник >100 задач → полный обход (полные данные)
- Rate limit: N×PUT → сериализация с семафором ≤50 req/min
- Частичный успех: часть перемещена, часть нет → отчёт `moved`/`failed`
- `dryRun` — не выполнять PUT

---

## 2. Summarize — сводка по доске

**Цель**: сформировать отчёт: метрики, группировка, рекомендации. Замена `daily_review.py`.

**Входы**:
- `boardId` (uuid)
- `format` (json|markdown) — по умолчанию markdown
- `since` (опц., timestamp) — дельта: только изменённое
- `groupBy` (опц.) — группировка (project, goal, context)

**Выходы** (markdown):
- TL;DR: N задач, M просрочено, K без стикеров, средний прогресс
- Таблица по колонкам: задачи / просрочка / средний прогресс
- Группировка по выбранному полю
- Рекомендации (по правилам: просрочка → в Review, etc.)

**API-эндпоинты**:
- `GET /columns?boardId=` — колонки
- `GET /tasks?boardId=&columnId=` — задачи по каждой колонке (полный обход)
- `GET /stickers?boardId=` — расшифровка стикеров

**Граничные случаи**:
- Пустая доска → «0 задач», без паники
- Задачи без стикеров → не падать, отметить в рекомендациях
- Пагинация: полный обход (приоритет — точность)
- Большой объём (200+ задач): маркдаун компактнее JSON (сводка, не сырьё)
- `since` — агрегировать только изменённые задачи

---

## 3. Audit — аудит доски

**Цель**: найти проблемы (просрочка, отсутствие стикеров) и опционально авто-исправить (переместить в Review).

**Входы**:
- `boardId` (uuid)
- `rules` (опц.) — какие проверки включить:
  - `overdue` (просрочка по deadline)
  - `missingStickers` (задачи без обязательных стикеров)
  - `autoMove` (перемещать просроченные в Review)
- `dryRun` (опц., bool) — показать проблемы без исправлений

**Выходы**:
- `issues` ([]): { type, taskId, description }
- `overdueCount`, `missingStickerCount`
- `autoMoved` (int) — сколько перемещено
- `summary` (markdown)

**API-эндпоинты**:
- `GET /columns?boardId=` — колонки (найти Review)
- `GET /tasks?boardId=&columnId=` — задачи (полный обход)
- `GET /stickers?boardId=` — обязательные стикеры
- `PUT /tasks/{id}` — только если `autoMove` и не `dryRun`

**Граничные случаи**:
- Нет колонки Review → не перемещать, отметить в отчёте
- Задача без deadline → не считать просроченной
- Просрочка определяется по `deadline.deadline < now_ms` (ms)
- `dryRun` — только чтение
- Частичный успех авто-перемещения → отчёт

---

## 4. Goal Tracking — прогресс целей

**Цель**: отследить прогресс целей по weighted KR (стикеры Weight и Progress).

**Входы**:
- `boardId` (uuid)
- `goalStickerId` (uuid) — стикер Client/Goal (строка)
- `weightStickerId` (uuid) — стикер Weight (number)
- `progressStickerId` (uuid) — стикер Progress (number)
- `format` (json|markdown)

**Выходы**:
- По каждой цели (значение goalSticker): список задач, вес, прогресс
- Weighted KR: сумма(weight × progress) / сумма(weight)
- Ранжирование по прогрессу
- `summary` (markdown)

**API-эндпоинты**:
- `GET /columns?boardId=`, `GET /tasks?boardId=&columnId=` (полный обход)
- `GET /stickers?boardId=` — расшифровка стикеров

**Граничные случаи**:
- Цель без задач → «нет данных», не падать
- Вес = 0 → исключить из знаменателя (деление на ноль)
- Задача без Progress → считать 0, отметить
- Дублирование задач между целями → не двойной счёт в общей сумме (уникальные taskIds)

---

## 5. Quick Actions — быстрое создание/перемещение/обновление задач

**Цель**: минимальные CRUD-операции для частых действий агента.

**Под-операции**:
- **Create**: title, description?, columnId?, stickers?, deadline?, assigned?
- **Move**: taskId + targetColumnId
- **Update**: taskId + любые поля (title, description, stickers, deadline, completed)
- **Get**: taskId

**Входы** (для Create):
- `title` (string, обяз.)
- `columnId`, `description`, `stickers` {stickerId: value}, `deadline`, `assigned`

**Выходы**:
- Создание: `{ id }` (новый taskId)
- Move/Update: `{ id, ok }`
- Get: полная задача

**API-эндпоинты**:
- `POST /tasks` — создать
- `PUT /tasks/{id}` — обновить/переместить
- `GET /tasks/{id}` — получить

**Граничные случаи**:
- Create без columnId → создать без колонки (если API позволяет)
- Move в несуществующую колонку → 404 → доменная ошибка ErrNotFound
- Update с `columnId: "-"` → снять с колонки
- Partial update: PUT только переданных полей (не затирать остальные)

---

## 6. Batch Stickers — массовое обновление стикеров

**Цель**: проставить/обновить стикер(ы) у группы задач одним действием.

**Входы**:
- `boardId` (uuid)
- `taskIds` ([]uuid) — целевые задачи
- `stickers` (map stickerId → value) — что проставить
- `dryRun` (опц.)

**Выходы**:
- `updated` (int), `failed` (int), `notFound` (int)
- `summary` (markdown)

**API-эндпоинты**:
- `PUT /tasks/{id}` с `{stickers: {...}}` — на каждую задачу
- `GET /stickers?boardId=` — валидация существования стикера (опц.)

**Граничные случаи**:
- Стикер не существует → доменная ошибка до массовой операции (проверить легенду первым)
- Пустой список задач → 0, без ошибок
- Rate limit: сериализация N×PUT
- Частичный успех → отчёт

---

## 7. Compression — цепочка сжатия ревью

**Цель**: daily → weekly → monthly → yearly сжатие ревью (из `review-compression.md`).

**Входы**:
- `level` (daily|weekly|monthly|yearly)
- `period` (дата/диапазон)
- `source` (опц.) — путь к предыдущему отчёту

**Выходы**:
- `summary` (markdown) — сжатый отчёт
- `savedTo` (опц.) — путь в memory/reviews/

**API-эндпоинты**:
- Чтение предыдущих отчётов (файловая система, не API)
- `GET /tasks...` — если нужна актуальная картина (по конфигу)

**Граничные случаи**:
- Нет предыдущего отчёта → создать с нуля
- Weekly/monthly выполняются в воскресенье / последний день месяца
- Формат хранения: `memory/reviews/YYYY-MM-DD-{level}.md` (как легаси)

---

## Сводная таблица

| Сценарий | Инструмент | Слой | Формат | Запросы | Мутации | Пагинация | since |
|----------|-----------|------|--------|---------|---------|-----------|-------|
| Bulk Move | `bulk_move_tasks` | 2 | JSON | 1 + N | N×PUT | полная | — |
| Summarize | `summarize_board` | 2 | markdown | 9–17 | — | полная | ✓ |
| Audit | `audit_board` | 2 | markdown | 9–17 | N×PUT (опц.) | полная | — |
| Goal Tracking | `track_goals` | 2 | markdown | 9–17 | — | полная | ✓ |
| Quick Actions | `create_task`/`update_task`/`get_task` | 1 | JSON | 1 | 1 | — | — |
| Batch Stickers | `batch_update_stickers` | 2 | JSON | 1 + N | N×PUT | — | — |
| Compression | `compress_reviews` | 2 | markdown | 0–17 | — | полная | — |

> Примечание: «9–17» — полный обход доски (колонки + задачи по колонкам). Если подтвердится `GET /tasks?boardId=` без `columnId` (неопределённость №6), станет 2–3 запроса. 

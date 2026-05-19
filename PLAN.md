# PLAN: YouGile MCP Server

**Цель**: Замена легаси OpenClaw-скиллов на Go MCP-сервер для ИИ-ассистента

---

## 1. API Reference

Полный справочник эндпоинтов, DTO и форматов данных: [`API_REFERENCE.md`](API_REFERENCE.md)

## 2. Go Package Structure

Инициализировать Go-модуль и определить структуру пакетов проекта в соответствии с DDD.

## 3. Domain Layer (DDD)

Определить:
- Value Objects (идентификаторы, заголовки, дедлайны, прогресс, приоритеты и т.д.)
- Entities (проект, доска, колонка, задача, стикер, пользователь)
- Aggregates (доска с колонками, задача с подзадачами, цель с KR)

## 4. Экспериментальная верификация API

После определения моделей данных (шаг 3) — выполнить реальные запросы к API для уточнения неопределённостей из `API_REFERENCE.md`:

- Проверить формат URL `/companies/{id}` — через слеш или нет
- Сравнить старый `/stickers` с новым `/string-stickers`: как они соотносятся
- Посмотреть реальное содержимое `stickers.custom` в ответе `GET /boards/{id}`
- Проверить формат тела ошибок (400, 401, 404, 429)

Результаты верификации — зафиксировать в `API_REFERENCE.md`.

## 5. Repositories

Определить интерфейсы репозиториев (domain) и их HTTP-реализации (infrastructure).
Охватить: проекты, доски, колонки, задачи, стикеры, пользователи. Учесть пагинацию.

## 6. Services & Events

Определить:
- Сервисы: BoardService, TaskService, ReviewService, AuditService, GoalService, CompressionService
- События: TaskCreated, TaskMoved, TaskCompleted, OverdueDetected, ReviewGenerated
- Как сервисы и события связаны между собой

## 7. MCP Tools

Сопоставить сервисы с MCP-инструментами, которые будет вызывать ИИ-ассистент.

## 8. Roadmap

1. HTTP-клиент (auth, rate limit, retry)
2. Репозитории (CRUD)
3. Базовые сервисы (Board, Task)
4. MCP-сервер (интеграция инструментов)
5. Аналитика (Review, Audit, Goal)
6. События и цепочки
7. Компрессия ревью

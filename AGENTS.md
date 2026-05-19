# AGENTS.md

## О проекте

MCP-сервер на Go для интеграции ИИ-ассистента с YouGile REST API v2.
Замена существующих OpenClaw-скиллов и Python-скриптов (`yougile_scripts/`).

## Ключевые факты

- **Язык:** Go 1.25
- **Архитектура:** DDD (Domain-Driven Design)
- **MCP SDK:** `github.com/mark3labs/mcp-go` (актуальная версия ~v0.54.0)
- **HTTP-клиент:** стандартный `net/http` + кастомный `RoundTripper` (Bearer auth, rate limiter)
- **Тестирование:** `go test` (стандартный)
- **Линтер:** `go vet`, планируется `golangci-lint`
- **Форматтер:** `gofmt` / `go fmt`

## Структура проекта

```
yougile-mcp/
├── cmd/
│   └── yougile-mcp/        # Точка входа (main.go)
├── internal/
│   ├── domain/             # DDD: value objects, entities, repository interfaces
│   │   ├── project/
│   │   ├── board/
│   │   ├── column/
│   │   ├── task/
│   │   ├── sticker/
│   │   ├── user/
│   │   └── valueobject/
│   ├── infrastructure/     # HTTP-клиент, реализации репозиториев
│   │   ├── http/
│   │   └── repository/
│   ├── service/            # Доменные сервисы
│   ├── event/              # In-memory event bus
│   └── mcp/                # MCP server, tools, handlers
├── API_REFERENCE.md        # Справочник YouGile API v2
├── LEGACY_PROJECT_STATE.md # Анализ легаси-кода
├── PLAN.md                 # План реализации
└── AGENTS.md               # Этот файл
```

## Важные замечания

- **Стиль кода:** стандартный Go (идиомы, ошибки как значения, без паники)
- **Обработка ошибок:** доменные ошибки через `errors.New` / `fmt.Errorf`; HTTP-статусы маппятся в доменные
- **Rate limiting:** 50 req/min, token bucket в `RoundTripper`
- **Пагинация:** ручное управление `offset`; `next` — boolean
- **Конфигурация:** API-ключ и companyId через переменные окружения (не в коде, не в репозитории)
- **Зависимости:** минимум внешних — только MCP SDK, всё остальное стандартная библиотека
- **Стикеры:** поддержка двух механизмов — старый `/stickers` и новый `/string-stickers` + `/sprint-stickers`

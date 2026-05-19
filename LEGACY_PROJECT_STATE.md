# LEGACY_PROJECT_STATE: yougile_scripts

> Дата анализа: 2026-05-19
> Репозиторий: `./yougile_scripts`
> Назначение: Набор OpenClaw-скиллов и Python-скриптов для работы с YouGile API v2 (Kanban-доски, задачи, цели по методологии Hybrid GTD/OKR)

---

## 1. Общая архитектура

```
yougile_scripts/
├── SKILL.md                          # Главный скилл "yougile" — CRUD-клиент YouGile API
├── config.json                       # Учётные данные (domain, companyId, apiKey)
├── .gitignore                        # Игнорирует всё (*)
├── references/
│   └── api-reference.md              # Полная Swagger-спека YouGile API v2
├── scripts/
│   ├── list-tasks.py                 # Получить все задачи доски (обход по колонкам)
│   └── create-board.py               # Создать доску + колонки
├── task-assistant/
│   ├── SKILL.md                      # Скилл "task-assistant" — ведущий GTD/OKR-процессов
│   ├── daily-review.md               # Полный протокол daily review (8 шагов)
│   ├── daily_review.py               # Python-скрипт daily review (хардкод)
│   ├── review-compression.md         # Цепочка сжатия daily→weekly→monthly→yearly
│   ├── templates.md                  # Шаблоны задач и целей
│   └── workflows.md                  # Описание воркфлоу (Capture, Review, Execution)
├── yougile-expert/
│   ├── SKILL.md                      # Скилл-консультант по YouGile (не делает API-вызовов)
│   └── references/
│       ├── api-endpoints.md          # Краткий справочник endpoints
│       └── productivity-workflows.md # Рекомендации по настройке GTD/OKR
└── yougile-audit/
    └── SKILL.md                      # Скилл аудита (просрочка, недостающие стикеры)
```

---

## 2. Технический стек

| Компонент | Технология |
|-----------|-----------|
| **Язык скриптов** | Python 3 (встроенная `urllib.request` или `requests`) |
| **Формат скиллов** | OpenClaw SKILL.md (YAML frontmatter + Markdown) |
| **Хранение данных** | Файловая система `memory/reviews/` (daily → weekly → monthly → yearly) |
| **API** | YouGile API v2, Base URL: `https://ru.yougile.com/api-v2` |
| **Аутентификация** | Bearer token в `config.json` (хардкод ключа в `daily_review.py`!) |
| **Rate Limit** | 50 запросов/мин → sleep 1.2с между вызовами |

---

## 3. Ключевые сущности YouGile API v2

| Сущность | Endpoint | Назначение |
|----------|----------|------------|
| Project | `GET/POST /projects` | Группа досок (есть дефолтный `<PROJECT_ID>...`) |
| Board | `GET/POST/PUT/DELETE /boards` | Kanban-доска с колонками |
| Column | `GET/POST /columns?boardId=` | Статусная колонка (Inbox, Next, Done...) |
| Task | `GET/POST/PUT/DELETE /tasks` | Карточка задачи (title, description, deadline, stickers) |
| Sticker | `GET/POST /stickers?boardId=` | Кастомное поле (select, number, date, string) |
| User | `GET /users?companyId=` | Участники компании |

---

## 4. Доска "Hybrid GTD/OKR"

**Проект**: Общий проект (`<PROJECT_ID>`)

**Колонки**:

| Колонка | ID | Назначение |
|---------|----|------------|
| Inbox | `0c0cad9d-...` | Захват всего нового |
| Next Actions | `1d42d32b-...` | Приоритетные задачи |
| In Progress | `d4931440-...` | В работе (WIP 3-5) |
| Waiting | `5db8c601-...` | Ожидание / планы |
| Delegate | `65a57983-...` | Делегировано |
| Done | `8d310601-...` | Выполнено |
| Review | `fae34734-...` | Просрочка / ревью |
| Events | `c6c9090b-...` | События / календарь |

**Стикеры** (custom fields):

| Стикер | ID | Тип | Значения |
|--------|----|-----|----------|
| Priority | `03f65e00-...` | select | High/Med/Low |
| Context | `f32bedde-...` | string | @Институт, @Дом, @Разработка |
| Energy | `0309f30f-...` | select | high/med/low |
| Progress | `42a0d7fd-...` | number | 0-100 |
| Weight | `0a0dd060-...` | number | 0-100 |
| Client/Goal | `6160dfcf-...` | string | Название цели |
| Project | `220fcfb0-...` | string | Категория проекта |

---

## 5. Существующие Python-скрипты

### 5.1 `scripts/list-tasks.py`
- **Назначение**: Получение всех активных задач доски
- **Логика**: Получает колонки доски → для каждой колонки запрашивает задачи
- **Проблемы**: Использует `requests` (внешняя зависимость), sleep 1.2с между запросами, нет обработки пагинации (limit=50 по умолчанию, но не передаёт)

### 5.2 `scripts/create-board.py`
- **Назначение**: Создание доски + колонок
- **Логика**: POST /boards → POST /columns для каждой колонки
- **Проблемы**: Использует `requests`, sleep 1.2с между колонками

### 5.3 `task-assistant/daily_review.py`
- **Назначение**: Ежедневный обзор доски (самый сложный скрипт, 296 строк)
- **Логика**:
  1. Проходит по всем 8 колонкам, собирает задачи
  2. Декодирует стикеры (Priority → эмодзи, Energy → эмодзи)
  3. Вычисляет метрики (просрочка, средний прогресс, etc.)
  4. Группирует по Project → Goal
  5. Вычисляет прогресс целей (weighted average KR)
  6. Выводит отформатированный отчёт
- **Проблемы**: Хардкод API-ключа и колонок (не читает config.json), использует `urllib.request` (не `requests`), нет сохранения в файл, нет пагинации (limit=100), sleep только 0.15с (может превысить rate limit)

---

## 6. Скиллы OpenClaw

### 6.1 `yougile` (главный SKILL.md)
- **Роль**: CRUD-клиент для YouGile API
- **Делегирует**: выполнение скриптов из `scripts/`
- **Rate limit**: 1200ms между вызовами
- **Default project**: `<PROJECT_ID>...`
- **Недостатки**: Нет скриптов для update/delete задач, нет работы со стикерами, нет пагинации

### 6.2 `task-assistant`
- **Роль**: Ведущий процессов GTD/OKR (захват → уточнение → создание → ревью)
- **Воркфлоу**: Capture → Clarify → Create → Daily Review → Compression chain
- **Review chain**: Daily → Weekly (Sun) → Monthly (last day) → Yearly (Dec 31)
- **Делегирует**: API-вызовы в `yougile`, консультации в `yougile-expert`

### 6.3 `yougile-expert`
- **Роль**: Консультант по YouGile (НЕ использует API)
- **Назначение**: Рекомендации по настройке досок, колонок, стикеров, методологий

### 6.4 `yougile-audit`
- **Роль**: Автоматический аудит доски
- **Действия**: Поиск просрочки → перемещение в Review, поиск задач без стикеров → запрос у пользователя

---

## 7. Известные проблемы (TODO)

1. **Хардкод секретов**: `daily_review.py` содержит API-ключ в открытом виде (строка 9), не читает `config.json`
2. **Разные HTTP-клиенты**: `scripts/` используют `requests`, `daily_review.py` использует `urllib.request` — нет единой абстракции
3. **Нет пагинации**: Ни один скрипт не обрабатывает `paging.next` — при >50-100 задач данные будут неполными
4. **Нет обработки ошибок**: Минимальная (только `try/except` в daily_review), нет retry-logic
5. **Rate limit хаос**: В SKILL.md указано 1200ms, в `daily_review.py` — 150ms (риск 429)
6. **Отсутствующие CRUD-операции**: Нет скриптов для update-task, delete-task, работы со стикерами, проектами, пользователями
7. **Нет кэширования**: Каждый daily review заново запрашивает все данные
8. **Нет сохранения отчётов**: daily_review.py выводит в stdout, но не пишет в `memory/reviews/`
9. **Разделение конфигов**: `SKILL.md` ссылается на `~/.openclaw/skills/yougile/config.json`, а `config.json` лежит рядом со скриптами

---

## 8. Что нужно от MCP

Исходя из легаси, MCP-сервер должен покрыть:

1. **Полный CRUD**: projects, boards, columns, tasks, stickers, users
2. **Пагинацию**: на всех endpoints
3. **Rate limiting**: Встроенный, конфигурируемый
4. **Единый HTTP-клиент**: С retry, обработкой ошибок, логированием
5. **Безопасность**: API-ключи через env, не в коде
6. **Замена daily_review.py**: Аналитика, группировка, метрики, рекомендации
7. **Экспорт**: Формирование отчётов (daily/weekly/monthly/yearly)

# Changelog

Все заметные изменения yougile-mcp. Формат — [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/),
версионирование — [SemVer](https://semver.org/lang/ru/). Правила ведения — [RELEASE_POLICY.md](RELEASE_POLICY.md).

История до v0.5.0 восстановлена ретроспективно по git; теги за этот период не выставлялись.

## [Unreleased]

## [0.7.1] — 2026-08-31

### Fixed
- Гонка `add_subtask`/`remove_subtask` (issue #7): параллельные вызовы на одного родителя теряли привязки (4 параллельных add → выживала 1). Read-modify-write теперь сериализуется per-parent мьютексом внутри процесса — покрывает и daemon-режим; внешние клиенты API по-прежнему вне контроля.

## [0.7.0] — 2026-08-31

### Added
- Восстановление удалённых задач (issue #6): `update_task` с `deleted: false` (раньше `delete_task` был однонаправленным, восстановление — только вручную в UI). `deleted: true` эквивалентен `delete_task`.

## [0.6.0] — 2026-08-31

### Added
- Подзадачи (issue #5): `create_task` с `subtasks` — атомарное создание родителя с детьми; `update_task` — `subtasks` (полная замена), `add_subtask` / `remove_subtask` (read-modify-write, идемпотентные, с проверкой существования ребёнка); `list_tasks` с `parentId` — прямые дети родителя + `broken` (битые ссылки).
- Инструмент `delete_task` — мягкое удаление задачи (deleted=true).
- `update_task`: `stickers: {id: null}` — снятие стикера (merge-семантика API: остальные не трогаются).

### Changed
- `list_tasks.boardId` больше не обязателен в схеме при передаче `parentId` (без обоих — ошибка).

### Fixed
- Формулировка про «полную замену стикеров» в `update_task` была неточной: API мержит по ключам, теперь null-значение явно снимает стикер.

## [0.5.0] — 2026-08-30 (ретроспектива, тег не выставлялся)

### Added
- Daemon-режим: `yougile-mcp serve --addr 127.0.0.1:7801` — один процесс обслуживает N агентов (streamable HTTP MCP на `/mcp`, health на `/healthz`).
- Общий rate-limiter демона (~50 rpm на всех агентов) вместо N независимых лимитов, пробивающих ключ YouGile.
- Идентичность и режим per-connection: имя агента из handshake (`clientInfo.name`), `set_mode` действует только на свою сессию и не трогает конфиг.
- Подпись `send_task_message` именем агента: `sender` > имя из handshake > `agent_id`/`YOUGILE_AGENT_ID`.
- Лог демона с атрибуцией: `agent=<имя> tool=<тул> ok|error`.
- pi-extension: подключение к демону через `YOUGILE_MCP_URL` / `mcp_url`, тихий fallback на stdio при недоступности, строка `connect:` в `/yougile-status`.

### Fixed
- Race режимов между агентами: `set_mode` в stdio персонально персистил общий конфиг и перетирал чужой режим (устранено session-scoped режимом в daemon; stdio-поведение для одиночного агента не изменилось).

## [0.4.0] — 2026-08-30 (ретроспектива, тег не выставлялся)

### Added
- `pi_scope.roots` в конфиге: список каталогов, где активно pi-расширение YouGile (по умолчанию — только каталог проекта).
- Команда `/yougile-on` — включение расширения на текущую сессию pi.

### Fixed
- Расширение больше не «выключает» чужие тулы pi при активации; вне roots тулы YouGile недоступны, а `/yougile-status` называет точную причину (cwd вне `pi_scope.roots`).

## [0.3.0] — 2026-08-23 (ретроспектива, тег не выставлялся)

### Added
- Чат задач: `get_task_messages` и `send_task_message` с обязательной идентификацией отправителя (префикс `[sender]`).
- Создание сущностей: `create_board`, `create_column` (цвет 1–7), `create_sticker` (string-stickers с состояниями, привязка к доске).
- Мягкое удаление досок: `delete_board`.
- Гайд деплоя на сервер OpenClaw (`DEPLOY_OPENCLAW.md`).

### Fixed
- `update_task`/`create_task` игнорировали аргумент `stickers`; опции легенды стикеров терялись, ID состояний (короткий hex) не проходили валидацию — исправлено.
- Вылет расширения pi: активация затирала встроенные тулы (bash/read/edit) — исправлено (#1).
- Зависание снапшота большой доски: последовательная загрузка колонок под rate-limit (~25–35с) заменена ограниченным параллелизмом — ~7с (#2).

### Performance
- Снапшот доски: bounded concurrency (4), дедлайн 45с, burst 10 для rate-limiter'а.

## [0.2.0] — 2026-08-17 (ретроспектива, тег не выставлялся)

### Added
- Три режима доступа: `read` / `confirm` / `yolo` (конфиг, `set_mode` через MCP API, `/yougile-mode` в pi).
- Политика инструментов `permissions.allow/confirm/deny` (glob), bulk-операции — dry-run-first.
- Аудит мутаций: JSONL-лог `~/.local/state/yougile-mcp/audit.jsonl`.
- По умолчанию `list_tasks`/снапшот возвращают только активные задачи (без completed/archived; включаются флагами).

### Security
- API-ключ перенесён из env в `~/.config/yougile-mcp/config.json` (права 600/700 обязательны, сервер отказывается стартовать с дырявыми правами; миграция через `yougile-mcp init`).

## [0.1.0] — 2026-08-17 (ретроспектива, тег не выставлялся)

### Added
- MVP MCP-сервера (stdio): DDD-слои, HTTP-клиент с Bearer-auth, token-bucket rate-limit (~50 rpm) и retry с backoff.
- 15 инструментов: CRUD задач/досок/колонок/стикеров, композитные сценарии — `get_board_snapshot`, `summarize_board`, `audit_board`, `track_goals`, `bulk_move_tasks`, `batch_update_stickers`, `compress_reviews`.
- Ручная пагинация `offset += limit` (API не поддерживает курсоры).

### Fixed
- Адаптация к реальному поведению YouGile API v2: `GET /tasks` не принимает `boardId`; легенда стикеров только через `/string-stickers`; старый `POST /stickers` мёртв.

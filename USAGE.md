# USAGE: работа с YouGile в pi

Краткое практическое руководство. Подробно: SECURITY.md (безопасность), AGENTS.md (архитектура).

---

## 1. Установка (один раз)

```bash
# 1) Собрать сервер и положить в ~/.local/bin
cd ~/wss/personal/yougile_mcp
make build
cp bin/yougile-mcp ~/.local/bin/

# 2) Расширение для pi (если ещё не установлено)
mkdir -p ~/.pi/agent/extensions/yougile-mcp
cp pi-extension/yougile-mcp/index.ts pi-extension/yougile-mcp/package.json ~/.pi/agent/extensions/yougile-mcp/
cd ~/.pi/agent/extensions/yougile-mcp && npm install

# 3) Конфиг с ключом (вне env): если есть YOUGILE_API_KEY в env — миграция
~/.local/bin/yougile-mcp init
# либо вручную: ~/.config/yougile-mcp/config.json (файл 600, каталог 700)

# 4) Удалить ключ из окружения (если был):
unset YOUGILE_API_KEY YOUGILE_LOGIN YOUGILE_PWD
```

Перезапустить pi (или `/reload` для перезагрузки расширений).

---

## 2. Проверка, что всё работает

Внутри pi:

```
/yougile-status
```

Покажет: режим (normal/read-only), путь конфига, наличие ключа, политику allow/confirm/deny, аудит.

Или просто спросить: «что у меня на доске Hybrid GTD/OKR?» — агент сам вызовет инструменты.

---

## 3. Что умеет агент (15 инструментов)

**Чтение (без вопросов, allow):**
- `list_projects`, `list_boards`, `list_columns`, `list_tasks` — списки
- `get_task`, `get_stickers`, `get_board_snapshot` — детали
- `summarize_board` — сводка (TL;DR + метрики + рекомендации)
- `track_goals` — прогресс целей (weighted KR)
- `compress_reviews` — сжатие отчётов (пишет локальные файлы)

**Изменения (спросит подтверждение, confirm):**
- `create_task` — создать задачу
- `update_task` — обновить/переместить/дедлайн/стикеры
- `bulk_move_tasks` — массовое перемещение
- `batch_update_stickers` — массовые стикеры
- `audit_board` — аудит (просрочка/стикеры/авто-перемещение в Review)

> **Важно**: мутационные инструменты запрашивают подтверждение в UI pi.
> Для массовых (`bulk_move_tasks`, `batch_update_stickers`, `audit_board` с autoMove)
> сначала показывается dry-run-план, потом вопрос «применить?».

---

## 4. Примеры запросов

```
«Покажи сводку по доске Hybrid GTD/OKR»
→ summarize_board (markdown, TL;DR)

«Что у меня просрочено?»
→ audit_board (dryRun по умолчанию) или summarize_board

«Создай задачу "Записаться к стоматологу" в колонку Inbox с дедлайном на пятницу»
→ create_task + подтверждение

«Перенеси все просроченные из Next Actions в Review»
→ bulk_move_tasks + dry-run-план + подтверждение

«Как продвигается цель X?»
→ track_goals

«Дай полный снимок доски»
→ get_board_snapshot
```

Агент сам выберет инструмент по описанию; можно указывать доску по названию — он найдёт ID через list_projects → list_boards.

---

## 5. Контроль безопасности (как пользователь)

| Что | Как |
|-----|-----|
| Ключ | `~/.config/yougile-mcp/config.json` (600), ротация — вручную в YouGile |
| Режим | `mode: read|confirm|yolo` в конфиге, через MCP `set_mode`, или `/yougile-mode` в pi |
| read | только чтение (мутации скрыты от LLM и блокируются сервером) |
| confirm | каждая мутация — диалог подтверждения; bulk — dry-run-first |
| yolo | все запросы разрешены без подтверждения |
| Политика | `permissions.allow/confirm/deny` (glob) в конфиге |
| История мутаций | `~/.local/state/yougile-mcp/audit.jsonl` |
| Текущее состояние | `/yougile-status` |

Пример политики в конфиге:

```json
"permissions": {
  "allow":   ["list_*", "get_*", "get_board_snapshot", "summarize_board", "track_goals", "compress_reviews"],
  "confirm": ["create_task", "update_task", "audit_board", "bulk_move_tasks", "batch_update_stickers"],
  "deny":    []
}
```

---

## 6. Если что-то не работает

1. `/yougile-status` — проверить ошибку подключения
2. Пересобрать сервер после изменений: `cd ~/wss/personal/yougile_mcp && make build && cp bin/yougile-mcp ~/.local/bin/`
3. `/reload` в pi (перезагрузить расширения)
4. Проверить права конфига: `stat -c "%a %n" ~/.config/yougile-mcp/config.json` → должно быть 600
5. Смотреть аудит: `tail ~/.local/state/yougile-mcp/audit.jsonl`

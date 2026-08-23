# Установка yougile-mcp на сервер OpenClaw

> Инструкция для агента/человека, выполняющего установку **на сервере** (root@<SERVER_IP>, linux amd64).
> Цель: подключить MCP-сервер yougile-mcp (YouGile API v2) к OpenClaw как stdio MCP-сервер.
> Предполагается, что файлы из «Шага 0» уже переданы на сервер (см. ниже).

---

## Шаг 0 — выполняется на рабочей станции (НЕ на сервере)

```bash
# 1) Собрать статический бинарь (уже собран: ~/wss/personal/yougile_mcp/dist/yougile-mcp-linux-amd64)
cd ~/wss/personal/yougile_mcp
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
  -o dist/yougile-mcp-linux-amd64 ./cmd/yougile-mcp

# 2) Передать на сервер (по одному SSH-каналу, ключ не светится в истории сервера):
scp dist/yougile-mcp-linux-amd64 root@<SERVER_IP>:/root/yougile-mcp
scp ~/.config/yougile-mcp/config.json root@<SERVER_IP>:/root/yougile-mcp-config.json
```

После этого всё дальнейшее — на сервере.

---

## Шаг 1 — разместить бинарь

```bash
mkdir -p /root/bin
mv /root/yougile-mcp /root/bin/yougile-mcp
chmod 700 /root/bin/yougile-mcp
/root/bin/yougile-mcp 2>&1 | head -1   # ожидание: "yougile-mcp: config: read ... " или про права — это ок, процесс ждёт stdio
```

## Шаг 2 — конфиг

```bash
mkdir -p /root/.config/yougile-mcp
chmod 700 /root/.config/yougile-mcp
mv /root/yougile-mcp-config.json /root/.config/yougile-mcp/config.json
chmod 600 /root/.config/yougile-mcp/config.json
```

Отредактировать `/root/.config/yougile-mcp/config.json`:

```json5
{
  "api_key": "...",                    // как передано, не менять
  "base_url": "https://ru.yougile.com/api-v2",
  "mode": "read",                      // ← ОБЯЗАТЕЛЬНО "read" на сервере (см. примечание о безопасности)
  "permissions": { ... },              // оставить как есть
  "audit": { "enabled": true }         // аудит мутаций включён
}
```

**Почему `mode: "read"`**: интерактивные подтверждения мутаций (`confirm`) — фича pi-расширения на рабочей станции; на сервере их никто не покажет, и сервер выполнит мутацию без вопросов. В `read` сервер жёстко блокирует все мутирующие инструменты (`create_task`, `update_task`, `bulk_move_tasks`, `batch_update_stickers`, `audit_board` с autoMove) — читать можно всё. Разблокировка позже: `set_mode` (MCP-инструмент) или правка конфига + рестарт.

## Шаг 3 — smoke-тест бинаря без OpenClaw

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
  | /root/bin/yougile-mcp
```

Ожидание (одна строка JSON):
```json
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"yougile-mcp","version":"0.1.0"}}}
```

Если вместо этого ошибка про права — см. Траблшутинг.

## Шаг 4 — подключить к OpenClaw

```bash
openclaw mcp add yougile --command /root/bin/yougile-mcp
openclaw mcp doctor yougile --probe
openclaw mcp reload    # если gateway уже запущен
```

Ожидание в `doctor --probe`: сервер отвечает, **17 инструментов**, 0 ошибок:
`list_projects, list_boards, list_columns, list_tasks, get_task, get_stickers, get_board_snapshot, summarize_board, audit_board, track_goals, bulk_move_tasks, batch_update_stickers, create_task, update_task, compress_reviews, get_mode, set_mode`

(мутирующие в read-режиме будут видны, но вызов вернёт «сервер в read-режиме: мутации запрещены» и запишется в аудит как `read-blocked`)

## Шаг 5 — выключить легаси YouGile-скиллы (если есть)

Проверить и убрать старые скиллы, чтобы не было двух путей к одному API (двойной rate limit, конкурирующая логика):

```bash
ls /root/.openclaw/skills/
# если есть yougile* / yougile-audit / review-compression и т.п.:
mkdir -p /root/legacy-skills-backup
mv /root/.openclaw/skills/yougile* /root/legacy-skills-backup/ 2>/dev/null
```

⚠️ В легаси `~/.openclaw/skills/yougile/config.json` лежит **старый API-ключ в открытом виде** — после переноса в backup файл с ключом удалить: `rm /root/legacy-skills-backup/*/config.json` (или весь backup после проверки). Ключ из него утёк в легаси-скрипты — по SECURITY.md подлежит ротации (делает владелец в UI YouGile).

## Шаг 6 — финальная проверка в чате OpenClaw

Спросить у агента в чате:

> «покажи сводку по доске Hybrid GTD/OKR»

Ожидание: `summarize_board` отвечает за ~7–10 секунд (полный снапшот доски, 10 колонок; дедлайн снапшота 45с зашит в сервер). Если OpenClaw обрежет по таймауту:

```bash
openclaw mcp configure yougile --timeout 60
```

---

## Траблшутинг

| Симптом | Причина / решение |
|---|---|
| `config: insecure file permissions ... require 600` | `chmod 600 ~/.config/yougile-mcp/config.json` и `chmod 700 ~/.config/yougile-mcp`. Сервер принципиально не стартует с дырявыми правами |
| `config not found at ...` в уведомлении | Конфиг не по дефолтному пути. Либо положить в `~/.config/yougile-mcp/config.json`, либо передать env при добавлении: `openclaw mcp add yougile --command /root/bin/yougile-mcp --env YOUGILE_CONFIG=/путь/к/config.json` |
| `binary not found` | `/root/bin/yougile-mcp` отсутствует — повторить Шаг 0/1 |
| 401 от YouGile | Ключ неверный/ротирован — обновить `api_key` в конфиге |
| Инструмент висит >60с | Должно уйти после `--timeout 60`; снапшот большой доски реально занимает ~7–10с (rate limit 50 rpm) |
| Неправильная просрочка | Сервер считает дедлайны по своему `time.Now()` — проверить TZ: `timedatectl`, при необходимости выставить корректный часовой пояс |

## Что где лежит (для справки)

| Путь | Что |
|---|---|
| `/root/bin/yougile-mcp` | MCP-сервер (статический Go-бинарь, linux amd64) |
| `/root/.config/yougile-mcp/config.json` | Конфиг с ключом (600), режим, политика, аудит |
| `~/.local/state/yougile-mcp/audit.jsonl` | JSONL-лог всех попыток мутаций (ok/error/read-blocked) |
| `~/.local/share/yougile-mcp/memory/reviews` | Директория отчётов `compress_reviews` |
| `~/.openclaw/openclaw.json` → `mcp.servers.yougile` | Регистрация сервера в OpenClaw |

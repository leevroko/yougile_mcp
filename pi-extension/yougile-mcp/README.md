# yougile-mcp — pi extension

Подключает Go MCP-сервер yougile-mcp к pi как расширение (stdio, официальный @modelcontextprotocol/sdk).

Рабочая копия живёт в `~/.pi/agent/extensions/yougile-mcp/`; эта — reference в репозитории.

Установка:
1. Скопировать в ~/.pi/agent/extensions/yougile-mcp/ (index.ts + package.json)
2. `npm install` внутри
3. Собрать сервер: `make build && cp bin/yougile-mcp ~/.local/bin/`
4. Конфиг: см. SECURITY.md (после реализации — ~/.config/yougile-mcp/config.json)

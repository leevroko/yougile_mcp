# yougile-mcp — Makefile

.PHONY: build test vet fmt run clean

build: ## Собрать бинарник
	go build -o bin/yougile-mcp ./cmd/yougile-mcp

test: ## Запустить тесты
	go test ./...

vet: ## Проверка статическим анализатором
	go vet ./...

fmt: ## Форматирование
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

run: ## Запуск MCP-сервера (stdio). Требует YOUGILE_API_KEY
	go run ./cmd/yougile-mcp

clean: ## Очистить бинарники
	rm -rf bin/

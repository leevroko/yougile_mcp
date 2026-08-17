// Command yougile-mcp — MCP-сервер для YouGile API v2.
package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/yougile-mcp/internal/infrastructure/http"
	"github.com/yougile-mcp/internal/infrastructure/repository"
	"github.com/yougile-mcp/internal/mcp"
	auditservice "github.com/yougile-mcp/internal/service/audit"
	boardservice "github.com/yougile-mcp/internal/service/board"
	compressionservice "github.com/yougile-mcp/internal/service/compression"
	goalservice "github.com/yougile-mcp/internal/service/goal"
	reviewservice "github.com/yougile-mcp/internal/service/review"
	taskservice "github.com/yougile-mcp/internal/service/task"
)

// envOrDefault возвращает значение переменной окружения или дефолт.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "yougile-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	apiKey := os.Getenv("YOUGILE_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("YOUGILE_API_KEY is required")
	}
	baseURL := envOrDefault("YOUGILE_BASE_URL", "https://ru.yougile.com/api-v2")
	memoryDir := envOrDefault("YOUGILE_MEMORY_DIR", "memory/reviews")

	// HTTP-клиент: BearerAuth → RateLimiter → Retry
	hc, err := http.NewClient(http.Config{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		MaxRetries: 3,
	})
	if err != nil {
		return fmt.Errorf("http client: %w", err)
	}

	// Репозитории
	projectRepo := repository.NewProjectRepository(hc, baseURL)
	boardRepo := repository.NewBoardRepository(hc, baseURL)
	columnRepo := repository.NewColumnRepository(hc, baseURL)
	taskRepo := repository.NewTaskRepository(hc, baseURL)
	stickerRepo := repository.NewStickerRepository(hc, baseURL)
	userRepo := repository.NewUserRepository(hc, baseURL)
	_ = userRepo // используется в будущем (snapshot пользователей)

	// Сервисы (прямые вызовы, без Event Bus)
	boards := boardservice.NewService(projectRepo, boardRepo, columnRepo, stickerRepo, taskRepo)
	tasks := taskservice.NewService(taskRepo, columnRepo, stickerRepo)
	review := reviewservice.NewService(boards)
	audit := auditservice.NewService(boards, tasks)
	goal := goalservice.NewService(boards)
	comp := compressionservice.NewService(memoryDir, review)

	// MCP-сервер
	srv := mcp.New(boards, tasks, review, audit, goal, comp)

	fmt.Fprintln(os.Stderr, "yougile-mcp: starting (stdio transport)")
	return server.ServeStdio(srv.MCPServer())
}

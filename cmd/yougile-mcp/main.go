// Command yougile-mcp — MCP-сервер для YouGile API v2.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/yougile-mcp/internal/audit"
	"github.com/yougile-mcp/internal/config"
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

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runInit(); err != nil {
			fmt.Fprintln(os.Stderr, "yougile-mcp init:", err)
			os.Exit(1)
		}
		return
	}

	cfgFlag := flag.String("config", "", "путь к config.json (по умолчанию ~/.config/yougile-mcp/config.json; env YOUGILE_CONFIG — тоже только путь)")
	flag.Parse()

	if err := run(*cfgFlag); err != nil {
		fmt.Fprintln(os.Stderr, "yougile-mcp:", err)
		os.Exit(1)
	}
}

// runInit — одноразовая миграция: ключ из YOUGILE_API_KEY → файл с правами 600.
func runInit() error {
	apiKey := os.Getenv("YOUGILE_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("YOUGILE_API_KEY не задан — мигрировать нечего. Экспортируйте его и повторите, либо создайте конфиг вручную")
	}
	path, err := config.Init(apiKey)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Конфиг создан: %s (права 600)\n", path)
	fmt.Fprintln(os.Stderr, "Теперь удалите YOUGILE_API_KEY (и YOUGILE_LOGIN/YOUGILE_PWD, если есть) из окружения.")
	return nil
}

func run(cfgFlag string) error {
	cfgPath, err := config.ResolvePath(cfgFlag)
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("config: api_key пуст в %s", cfgPath)
	}

	// HTTP-клиент: BearerAuth → RateLimiter → Retry
	// Burst=10: страницам снапшота не выстраиваться в очередь по 1.2с
	// (issue #2); суммарный лимит остаётся ~50 rpm.
	hc, err := http.NewClient(http.Config{
		BaseURL:    cfg.BaseURL,
		APIKey:     cfg.APIKey,
		Burst:      10,
		MaxRetries: 3,
	})
	if err != nil {
		return fmt.Errorf("http client: %w", err)
	}

	// Репозитории
	projectRepo := repository.NewProjectRepository(hc, cfg.BaseURL)
	boardRepo := repository.NewBoardRepository(hc, cfg.BaseURL)
	columnRepo := repository.NewColumnRepository(hc, cfg.BaseURL)
	taskRepo := repository.NewTaskRepository(hc, cfg.BaseURL)
	stickerRepo := repository.NewStickerRepository(hc, cfg.BaseURL)
	chatRepo := repository.NewChatRepository(hc, cfg.BaseURL)

	// Сервисы (прямые вызовы, без Event Bus)
	boards := boardservice.NewService(projectRepo, boardRepo, columnRepo, stickerRepo, taskRepo)
	tasks := taskservice.NewService(taskRepo, columnRepo, stickerRepo, chatRepo)
	review := reviewservice.NewService(boards)
	auditSvc := auditservice.NewService(boards, tasks)
	goal := goalservice.NewService(boards)
	comp := compressionservice.NewService(cfg.MemoryDir, review)

	// Аудит мутаций (JSONL)
	auditLog := audit.NewFileLogger(cfg.Audit.Path)
	if !cfg.Audit.Enabled {
		auditLog = audit.NewNoopLogger()
	}

	// MCP-сервер
	srv := mcp.New(mcp.Deps{
		Board: boards, Tasks: tasks, Review: review,
		AuditSvc: auditSvc, Goal: goal, Compression: comp,
		ReadOnly:   cfg.ReadOnly,
		Mode:       cfg.Mode,
		Config:     &cfg,
		ConfigPath: cfgPath,
		AuditLog:   auditLog,
	})

	mode := string(cfg.Mode)
	if cfg.ReadOnly {
		mode = "read-only"
	}
	fmt.Fprintf(os.Stderr, "yougile-mcp: starting (stdio, mode=%s, config=%s)\n", mode, cfgPath)
	return server.ServeStdio(srv.MCPServer())
}

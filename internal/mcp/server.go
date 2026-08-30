// Package mcp — MCP-сервер: инструменты двух слоёв + markdown-презентация.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/yougile-mcp/internal/audit"
	"github.com/yougile-mcp/internal/config"
	"github.com/yougile-mcp/internal/domain/board"
	"github.com/yougile-mcp/internal/domain/sticker"
	"github.com/yougile-mcp/internal/domain/task"
	"github.com/yougile-mcp/internal/domain/valueobject"
	auditservice "github.com/yougile-mcp/internal/service/audit"
	boardservice "github.com/yougile-mcp/internal/service/board"
	compressionservice "github.com/yougile-mcp/internal/service/compression"
	goalservice "github.com/yougile-mcp/internal/service/goal"
	reviewservice "github.com/yougile-mcp/internal/service/review"
	taskservice "github.com/yougile-mcp/internal/service/task"
)

// Deps — зависимости MCP-сервера.
type Deps struct {
	Board       boardservice.Service
	Tasks       taskservice.Service
	Review      reviewservice.Service
	AuditSvc    auditservice.Service
	Goal        goalservice.Service
	Compression compressionservice.Service
	ReadOnly    bool           // legacy: read-only режим (мутации скрыты)
	Mode        config.Mode    // read | confirm | yolo
	Config      *config.Config // полный загруженный конфиг (для персиста режима без потери полей)
	ConfigPath  string         // путь к конфигу для персиста режима
	AuditLog    audit.Logger   // аудит мутаций (nil → заглушка)
	Daemon      bool           // issue #4: один сервер на N агентов — per-session режим и идентичность
}

// Server — MCP-сервер YouGile.
type Server struct {
	mcp      *server.MCPServer
	board    boardservice.Service
	tasks    taskservice.Service
	review   reviewservice.Service
	audit    auditservice.Service
	goal     goalservice.Service
	comp     compressionservice.Service
	readOnly bool // legacy flag: если true — мутации скрыты
	mode     config.Mode
	agentID  string            // идентификатор агента (YOUGILE_AGENT_ID/agent_id) для префиксов в чатах
	cfgPath  string
	cfg      *config.Config
	auditLog audit.Logger
	sessions *SessionRegistry // per-session режим/имя (актуально в daemon-режиме)
	daemon   bool
	mu       sync.RWMutex
}

// Mode возвращает текущий режим.
func (s *Server) Mode() config.Mode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

// SetMode изменяет режим и персистит в конфиг-файл (stdio-режим).
func (s *Server) SetMode(m config.Mode) bool {
	if !config.ValidMode(string(m)) {
		return false
	}
	s.mu.Lock()
	s.mode = m
	if s.cfgPath != "" {
		s.cfg.Mode = m
		if err := s.cfg.SaveMode(m, s.cfgPath); err != nil {
			s.mu.Unlock()
			return false
		}
	}
	s.mu.Unlock()
	return true
}

// EffectiveMode — режим для конкретного вызова: в daemon-режиме пер-сессионный
// override агента, если задан; иначе глобальный режим сервера (issue #4).
func (s *Server) EffectiveMode(ctx context.Context) config.Mode {
	if s.daemon {
		if st, ok := s.sessions.Get(SessionIDFromCtx(ctx)); ok && st.Mode != "" {
			return st.Mode
		}
	}
	return s.Mode()
}

// AgentName — идентичность агента для текущего вызова (issue #4):
// в daemon-режиме — имя из handshake сессии; иначе — пусто.
func (s *Server) AgentName(ctx context.Context) string {
	if s.daemon {
		if st, ok := s.sessions.Get(SessionIDFromCtx(ctx)); ok {
			return st.Name
		}
	}
	return ""
}

type sessionKey struct{}

// SessionIDFromCtx извлекает ID MCP-сессии из контекста вызова
// (server.ClientSessionFromContext). Пустая строка — сессия неизвестна.
func SessionIDFromCtx(ctx context.Context) string {
	if cs := server.ClientSessionFromContext(ctx); cs != nil {
		return cs.SessionID()
	}
	return ""
}

// New создаёт MCP-сервер с зарегистрированными инструментами.
func New(deps Deps) *Server {
	log := deps.AuditLog
	if log == nil {
		log = audit.NewNoopLogger()
	}
	cfg := deps.Config
	if cfg == nil {
		cfg = &config.Config{}
	}
	if deps.Mode != "" {
		cfg.Mode = deps.Mode
	}
	// legacy read_only=true → mode=read; иначе дефолт confirm
	if deps.ReadOnly || cfg.Mode == config.ModeRead {
		cfg.Mode = config.ModeRead
	}
	if cfg.Mode == "" {
		cfg.Mode = config.ModeConfirm
	}
	// issue #4: в daemon-режиме агент представляется в handshake (clientInfo.name) →
	// per-session режим + идентичность отправителя без общих гонок за config.json.
	// Хуки задаются опцией NewMCPServer(WithHooks), поэтому собираем до создания ядра.
	var opts []server.ServerOption = []server.ServerOption{server.WithToolCapabilities(false)}
	sessions := newSessionRegistry()
	if deps.Daemon {
		hooks := &server.Hooks{}
		hooks.AddAfterInitialize(func(ctx context.Context, _ any, req *mcp.InitializeRequest, _ *mcp.InitializeResult) {
			sessions.Remember(SessionIDFromCtx(ctx), req.Params.ClientInfo.Name)
		})
		hooks.AddOnUnregisterSession(func(ctx context.Context, session server.ClientSession) {
			sessions.Forget(session.SessionID())
		})
		hooks.AddAfterCallTool(func(ctx context.Context, _ any, req *mcp.CallToolRequest, result any) {
			name := "unknown"
			if st, ok := sessions.Get(SessionIDFromCtx(ctx)); ok && st.Name != "" {
				name = st.Name
			}
			outcome := "ok"
			if res, ok := result.(*mcp.CallToolResult); ok && res != nil && res.IsError {
				outcome = "error"
			}
			fmt.Fprintf(os.Stderr, "yougile-mcp: agent=%s tool=%s %s\n", name, req.Params.Name, outcome)
		})
		opts = append(opts, server.WithHooks(hooks))
	}
	s := &Server{
		mcp:      server.NewMCPServer("yougile-mcp", "0.1.0", opts...),
		board:    deps.Board,
		tasks:    deps.Tasks,
		review:   deps.Review,
		audit:    deps.AuditSvc,
		goal:     deps.Goal,
		comp:     deps.Compression,
		readOnly: deps.ReadOnly,
		mode:     cfg.Mode,
		agentID:  cfg.AgentID,
		cfgPath:  deps.ConfigPath,
		cfg:      cfg,
		auditLog: log,
		sessions: sessions,
		daemon:   deps.Daemon,
	}
	s.registerTools()
	return s
}

// MCPServer возвращает внутренний MCP-сервер (для транспортов).
func (s *Server) MCPServer() *server.MCPServer { return s.mcp }

// registerTools регистрирует инструменты двух слоёв.
// В read-only режиме мутационные инструменты не регистрируются вообще.
func (s *Server) registerTools() {
	ro := func(opts ...mcp.ToolOption) []mcp.ToolOption {
		return append(opts, mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithIdempotentHintAnnotation(true))
	}
	mut := func(opts ...mcp.ToolOption) []mcp.ToolOption {
		return append(opts, mcp.WithReadOnlyHintAnnotation(false), mcp.WithDestructiveHintAnnotation(true))
	}
	register := s.mcp.AddTool
	mutating := func(tool mcp.Tool, name string, h server.ToolHandlerFunc) {
		// Всегда регистрируем (даже в read) — блокировка в wrapMutating по режиму.
		// Сокрытие от LLM — на стороне pi-расширения (setActiveTools).
		s.mcp.AddTool(tool, s.wrapMutating(name, h))
	}

	// ── Режимы: set_mode / get_mode (всегда доступны, даже в read) ──
	register(mcp.NewTool("get_mode",
		ro(mcp.WithDescription("Текущий режим доступа: read | confirm | yolo"))...,
	), s.handleGetMode)

	mutating(mcp.NewTool("set_mode",
		mut(mcp.WithDescription("Сменить режим: read (только чтение) | confirm (запись с подтверждением) | yolo (без подтверждений)"),
			mcp.WithString("mode", mcp.Required(), mcp.Description("read | confirm | yolo")),
		)...,
	), "set_mode", s.handleSetMode)

	// ── Слой 1: тонкий CRUD ──
	register(mcp.NewTool("list_projects",
		ro(mcp.WithDescription("Список проектов"))...,
	), s.handleListProjects)

	register(mcp.NewTool("list_boards",
		ro(mcp.WithDescription("Список досок проекта"),
			mcp.WithString("projectId", mcp.Required(), mcp.Description("ID проекта")))...,
	), s.handleListBoards)

	register(mcp.NewTool("list_columns",
		ro(mcp.WithDescription("Список колонок доски"),
			mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")))...,
	), s.handleListColumns)

	register(mcp.NewTool("list_tasks",
		ro(mcp.WithDescription("Задачи доски/колонки + названия колонок + легенда стикеров. С parentId — прямые дети родителя (композитно: parent → его subtasks → задачи; broken — ID несуществующих)"),
			mcp.WithString("boardId", mcp.Description("ID доски (обязателен без parentId)")),
			mcp.WithString("columnId", mcp.Description("ID колонки (опционально)")),
			mcp.WithString("parentId", mcp.Description("ID родителя: вернуть его прямых подзадачи (boardId игнорируется)")),
			mcp.WithBoolean("includeCompleted", mcp.Description("Включить выполненные (по умолчанию false)")),
			mcp.WithBoolean("includeArchived", mcp.Description("Включить архивированные (по умолчанию false)")),
			mcp.WithString("format", mcp.Description("Формат: json|markdown (по умолчанию json)")))...,
	), s.handleListTasks)

	register(mcp.NewTool("get_task",
		ro(mcp.WithDescription("Задача по ID"),
			mcp.WithString("taskId", mcp.Required(), mcp.Description("ID задачи")))...,
	), s.handleGetTask)

	mutating(mcp.NewTool("delete_task",
		mut(mcp.WithDescription("Мягкое удаление задачи (deleted=true). Задача исчезает из списков; восстановление — вручную в UI"),
			mcp.WithString("taskId", mcp.Required(), mcp.Description("ID задачи")))...,
	), "delete_task", s.handleDeleteTask)

	mutating(mcp.NewTool("create_task",
		mut(mcp.WithDescription("Создание задачи"),
			mcp.WithString("title", mcp.Required(), mcp.Description("Название")),
			mcp.WithString("columnId", mcp.Description("ID колонки")),
			mcp.WithString("description", mcp.Description("Описание")),
			mcp.WithNumber("deadline", mcp.Description("Дедлайн (timestamp ms)")),
			mcp.WithObject("stickers", mcp.Description("Стикеры: {stickerId: значение}. Для select — ID опции из get_stickers")),
			mcp.WithArray("subtasks", mcp.Items(map[string]any{"type": "string"}), mcp.Description("ID дочерних задач — родитель создаётся сразу с детьми")))...,
	), "create_task", s.handleCreateTask)

	mutating(mcp.NewTool("update_task",
		mut(mcp.WithDescription("Обновление задачи (перемещение, стикеры, дедлайн, подзадачи)"),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithString("taskId", mcp.Required(), mcp.Description("ID задачи")),
			mcp.WithString("columnId", mcp.Description("Целевая колонка (перемещение)")),
			mcp.WithString("title", mcp.Description("Новое название")),
			mcp.WithString("description", mcp.Description("Новое описание")),
			mcp.WithNumber("deadline", mcp.Description("Дедлайн (timestamp ms)")),
			mcp.WithBoolean("completed", mcp.Description("Выполнена")),
			mcp.WithObject("stickers", mcp.Description("Стикеры: {stickerId: значение|null}. Значение null — снять стикер (merge: остальные не трогаются). Для select — ID опции из get_stickers")),
			mcp.WithArray("subtasks", mcp.Items(map[string]any{"type": "string"}), mcp.Description("ПОЛНАЯ ЗАМЕНА списка дочерних задач (массив ID). Нельзя смешивать с add_subtask/remove_subtask")),
			mcp.WithString("add_subtask", mcp.Description("Привязать задачу как подзадачу (ID). Проверяется существование; повтор — no-op")),
			mcp.WithString("remove_subtask", mcp.Description("Отвязать подзадачу (ID). Задача не удаляется; повтор — no-op")))...,
	), "update_task", s.handleUpdateTask)

	mutating(mcp.NewTool("create_board",
		mut(mcp.WithDescription("Создание доски в проекте"),
			mcp.WithString("projectId", mcp.Required(), mcp.Description("ID проекта (из list_projects)")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Название доски")))...,
	), "create_board", s.handleCreateBoard)

	mutating(mcp.NewTool("create_column",
		mut(mcp.WithDescription("Создание колонки на доске"),
			mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Название колонки")),
			mcp.WithNumber("color", mcp.Description("Цвет: 1=default, 2=gray, 3=blue, 4=green, 5=orange, 6=red, 7=purple")))...,
	), "create_column", s.handleCreateColumn)

	mutating(mcp.NewTool("create_sticker",
		mut(mcp.WithDescription("Создание стикера с набором состояний (select). Опционально сразу привязать к доске"),
			mcp.WithString("name", mcp.Required(), mcp.Description("Название стикера")),
			mcp.WithString("icon", mcp.Description("Иконка (необязательно, например prio, star)")),
			mcp.WithString("boardId", mcp.Description("ID доски: привязать стикер к доске сразу после создания")),
			mcp.WithArray("states", mcp.Description("Состояния: [{name, color}] (color hex, необязателен)")))...,
	), "create_sticker", s.handleCreateSticker)

	register(mcp.NewTool("get_task_messages",
		ro(mcp.WithDescription("Сообщения чата задачи (taskId = chatId). Пагинация limit/offset"),
			mcp.WithString("taskId", mcp.Required(), mcp.Description("ID задачи")),
			mcp.WithNumber("limit", mcp.Description("Максимум сообщений (по умолчанию 50, максимум 100)")),
			mcp.WithNumber("offset", mcp.Description("Смещение пагинации")))...,
	), s.handleGetTaskMessages)

	mutating(mcp.NewTool("send_task_message",
		mut(mcp.WithDescription("Отправить сообщение в чат задачи. К тексту ВСЕГДА добавляется префикс идентификации: \"[sender] текст\""),
			mcp.WithString("taskId", mcp.Required(), mcp.Description("ID задачи")),
			mcp.WithString("text", mcp.Required(), mcp.Description("Текст сообщения (без префикса — он добавится автоматически)")),
			mcp.WithString("sender", mcp.Description("Кто пишет (имя агента, напр. \"pi/yougile\"). Если не задан — имя клиента из handshake (daemon) или agent_id сервера; иначе ошибка")))...,
	), "send_task_message", s.handleSendTaskMessage)

	mutating(mcp.NewTool("delete_board",
		mut(mcp.WithDescription("Удаление доски (мягкое, deleted=true). Колонки и задачи остаются в API, но доска исчезает из списков"),
			mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")))...,
	), "delete_board", s.handleDeleteBoard)

	register(mcp.NewTool("get_stickers",
		ro(mcp.WithDescription("Легенда стикеров доски"),
			mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")))...,
	), s.handleGetStickers)

	// ── Слой 2: композитные ──
	register(mcp.NewTool("get_board_snapshot",
		ro(mcp.WithDescription("Полное состояние доски: колонки, задачи, стикеры. По умолчанию только активные задачи"),
			mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
			mcp.WithNumber("since", mcp.Description("Дельта: timestamp ms, только изменённые")),
			mcp.WithBoolean("includeCompleted", mcp.Description("Включить выполненные (по умолчанию false)")),
			mcp.WithBoolean("includeArchived", mcp.Description("Включить архивированные (по умолчанию false)")),
			mcp.WithString("format", mcp.Description("Формат: json|markdown")))...,
	), s.handleGetBoardSnapshot)

	register(mcp.NewTool("summarize_board",
		ro(mcp.WithDescription("Сводка: TL;DR + метрики + группировка + рекомендации"),
			mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
			mcp.WithNumber("since", mcp.Description("Дельта: timestamp ms")),
			mcp.WithString("format", mcp.Description("Формат: json|markdown (по умолчанию markdown)")))...,
	), s.handleSummarizeBoard)

	mutating(mcp.NewTool("audit_board",
		mut(mcp.WithDescription("Аудит: просрочка, отсутствие стикеров, авто-перемещение"),
			mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
			mcp.WithBoolean("overdue", mcp.Description("Проверка просрочки")),
			mcp.WithBoolean("missingStickers", mcp.Description("Проверка стикеров")),
			mcp.WithBoolean("autoMove", mcp.Description("Перемещать просроченные в Review")),
			mcp.WithBoolean("dryRun", mcp.Description("Только чтение, без изменений")),
			mcp.WithString("format", mcp.Description("Формат: json|markdown (по умолчанию markdown)")))...,
	), "audit_board", s.handleAuditBoard)

	register(mcp.NewTool("track_goals",
		ro(mcp.WithDescription("Прогресс целей (weighted KR)"),
			mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
			mcp.WithNumber("since", mcp.Description("Дельта: timestamp ms")),
			mcp.WithString("format", mcp.Description("Формат: json|markdown (по умолчанию markdown)")))...,
	), s.handleTrackGoals)

	mutating(mcp.NewTool("bulk_move_tasks",
		mut(mcp.WithDescription("Массовое перемещение задач"),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
			mcp.WithString("sourceColumnId", mcp.Description("Колонка-источник (все задачи)")),
			mcp.WithString("targetColumnId", mcp.Required(), mcp.Description("Целевая колонка")),
			mcp.WithBoolean("dryRun", mcp.Description("Только чтение")))...,
	), "bulk_move_tasks", s.handleBulkMove)

	mutating(mcp.NewTool("batch_update_stickers",
		mut(mcp.WithDescription("Массовое обновление стикеров"),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
			mcp.WithBoolean("dryRun", mcp.Description("Только чтение")))...,
	), "batch_update_stickers", s.handleBatchStickers)

	register(mcp.NewTool("compress_reviews",
		ro(mcp.WithDescription("Сжатие ревью (daily→weekly→...) — пишет только локальные файлы отчётов"),
			mcp.WithString("level", mcp.Required(), mcp.Description("daily|weekly|monthly|yearly")),
			mcp.WithNumber("from", mcp.Required(), mcp.Description("Начало периода (ms)")),
			mcp.WithNumber("to", mcp.Required(), mcp.Description("Конец периода (ms)")))...,
	), s.handleCompress)
}

// wrapMutating оборачивает мутационный хендлер: аудит + блокировка в read-режиме.
func (s *Server) wrapMutating(name string, h server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		// read-режим блокирует мутации, кроме set_mode (иначе не выйти из read).
		if s.EffectiveMode(ctx) == config.ModeRead && name != "set_mode" {
			res := errResult(errors.New("сервер в read-режиме: мутации запрещены. Вызовите set_mode для переключения"))
			s.auditLog.Log(name, "read-blocked", args)
			return res, nil
		}
		res, err := h(ctx, req)
		// dryRun-вызовы — не мутации, в аудит не пишем
		if !boolVal(args, "dryRun") {
			outcome := "ok"
			if err != nil || strings.HasPrefix(firstText(res), "Ошибка") {
				outcome = "error"
			}
			s.auditLog.Log(name, outcome, args)
		}
		return res, err
	}
}

// firstText возвращает первый текстовый контент результата.
func firstText(res *mcp.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	if c, ok := res.Content[0].(mcp.TextContent); ok {
		return c.Text
	}
	return ""
}

// ── Хендлеры слоя 1 ──

func (s *Server) handleListProjects(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projects, _, err := s.board.ListProjects(ctx)
	if err != nil {
		return errResult(err), nil
	}
	data, err := toJSON(projects)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

func (s *Server) handleListBoards(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	pid, err := parseProjectID(str(args, "projectId"))
	if err != nil {
		return errResult(err), nil
	}
	boards, _, err := s.board.ListBoards(ctx, pid)
	if err != nil {
		return errResult(err), nil
	}
	data, err := toJSON(boards)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

func (s *Server) handleListColumns(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	bid, err := parseBoardID(str(args, "boardId"))
	if err != nil {
		return errResult(err), nil
	}
	cols, err := s.board.ListColumns(ctx, bid)
	if err != nil {
		return errResult(err), nil
	}
	data, err := toJSON(cols)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

func (s *Server) handleListTasks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	// parentId: вернуть прямых детей родителя (композитно — API не умеет фильтровать
	// по родителю, связь хранится только на родителе; см. API_REFERENCE.md §6).
	if v := str(args, "parentId"); v != "" {
		pid, err := parseTaskID(v)
		if err != nil {
			return errResult(err), nil
		}
		res, err := s.tasks.ListSubtasks(ctx, pid)
		if err != nil {
			return errResult(err), nil
		}
		data, err := toJSON(map[string]any{
			"parent": res.Parent,
			"tasks":  res.Tasks,
			"broken": res.Broken,
		})
		if err != nil {
			return errResult(err), nil
		}
		return textResult(data), nil
	}

	bidRaw := str(args, "boardId")
	if bidRaw == "" {
		return errResult(errors.New("boardId обязателен (или используйте parentId для подзадач)")), nil
	}
	bid, err := parseBoardID(bidRaw)
	if err != nil {
		return errResult(err), nil
	}
	var cid *valueobject.ColumnID
	if v := str(args, "columnId"); v != "" {
		c, err := parseColumnID(v)
		if err != nil {
			return errResult(err), nil
		}
		cid = &c
	}
	res, err := s.tasks.ListTasksFiltered(ctx, bid, cid, taskservice.TaskFilter{
		IncludeCompleted: boolVal(args, "includeCompleted"),
		IncludeArchived:  boolVal(args, "includeArchived"),
	})
	if err != nil {
		return errResult(err), nil
	}

	f := parseFormat(str(args, "format"))
	if f == formatMarkdown {
		return textResult(renderTasksMarkdown(res)), nil
	}
	data, err := toJSON(res)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

func (s *Server) handleGetTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	tid, err := parseTaskID(str(args, "taskId"))
	if err != nil {
		return errResult(err), nil
	}
	t, err := s.tasks.GetTask(ctx, tid)
	if err != nil {
		return errResult(err), nil
	}
	data, err := toJSON(t)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

func (s *Server) handleDeleteTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	tid, err := parseTaskID(str(args, "taskId"))
	if err != nil {
		return errResult(err), nil
	}
	if err := s.tasks.DeleteTask(ctx, tid); err != nil {
		return errResult(err), nil
	}
	return textResult(`{"ok": true, "deleted": true}`), nil
}

func (s *Server) handleCreateTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	params := taskservice.CreateTaskParams{Title: str(args, "title")}
	if v := str(args, "columnId"); v != "" {
		c, err := parseColumnID(v)
		if err != nil {
			return errResult(err), nil
		}
		params.ColumnID = &c
	}
	if d := numInt(args, "deadline"); d > 0 {
		dl, err := valueobject.NewDeadline(int64(d))
		if err != nil {
			return errResult(err), nil
		}
		params.Deadline = &dl
	}
	if st, clear, err := parseStickers(args, "stickers"); err != nil {
		return errResult(err), nil
	} else if len(st) > 0 {
		params.Stickers = st
	} else if len(clear) > 0 {
		return errResult(errors.New("stickers: null при создании не имеет смысла — задача ещё без стикеров, просто не указывайте ключ")), nil
	}
	for _, raw := range strSlice(args, "subtasks") {
		sid, err := parseTaskID(raw)
		if err != nil {
			return errResult(fmt.Errorf("subtasks: %w", err)), nil
		}
		params.Subtasks = append(params.Subtasks, sid)
	}
	params.Description = str(args, "description")

	tid, err := s.tasks.CreateTask(ctx, params)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(fmt.Sprintf(`{"id": %q}`, tid.String())), nil
}

func (s *Server) handleUpdateTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	tid, err := parseTaskID(str(args, "taskId"))
	if err != nil {
		return errResult(err), nil
	}
	ur := task.UpdateRequest{}
	if v := str(args, "columnId"); v != "" {
		c, err := parseColumnID(v)
		if err != nil {
			return errResult(err), nil
		}
		ur.ColumnID = &c
	}
	if v := str(args, "title"); v != "" {
		ur.Title = &v
	}
	if v := str(args, "description"); v != "" {
		ur.Description = &v
	}
	if d := numInt(args, "deadline"); d > 0 {
		dl, err := valueobject.NewDeadline(int64(d))
		if err != nil {
			return errResult(err), nil
		}
		ur.Deadline = &dl
	}
	if v, ok := args["completed"].(bool); ok {
		ur.Completed = &v
	}
	if st, clear, err := parseStickers(args, "stickers"); err != nil {
		return errResult(err), nil
	} else if st != nil || clear != nil {
		ur.Stickers = &st
		ur.ClearStickers = clear
	}

	// Подзадачи (issue #5): полная замена subtasks или точечные add/remove
	// (read-modify-write в сервисе). Одновременно — нельзя.
	hasSubtasks := strSlice(args, "subtasks")
	addSub := str(args, "add_subtask")
	removeSub := str(args, "remove_subtask")
	if len(hasSubtasks) > 0 && (addSub != "" || removeSub != "") {
		return errResult(errors.New("subtasks нельзя смешивать с add_subtask/remove_subtask: либо полный список, либо точечное изменение")), nil
	}
	if addSub != "" && removeSub != "" {
		return errResult(errors.New("add_subtask и remove_subtask в одном вызове: выполните двумя вызовами")), nil
	}
	if len(hasSubtasks) > 0 {
		ids := make([]valueobject.TaskID, 0, len(hasSubtasks))
		for _, raw := range hasSubtasks {
			sid, err := parseTaskID(raw)
			if err != nil {
				return errResult(fmt.Errorf("subtasks: %w", err)), nil
			}
			ids = append(ids, sid)
		}
		ur.Subtasks = &ids
	}
	if err := s.tasks.UpdateTask(ctx, tid, ur); err != nil {
		return errResult(err), nil
	}
	if addSub != "" {
		cid, err := parseTaskID(addSub)
		if err != nil {
			return errResult(fmt.Errorf("add_subtask: %w", err)), nil
		}
		if err := s.tasks.AddSubtask(ctx, tid, cid); err != nil {
			return errResult(err), nil
		}
	}
	if removeSub != "" {
		cid, err := parseTaskID(removeSub)
		if err != nil {
			return errResult(fmt.Errorf("remove_subtask: %w", err)), nil
		}
		if err := s.tasks.RemoveSubtask(ctx, tid, cid); err != nil {
			return errResult(err), nil
		}
	}
	return textResult(`{"ok": true}`), nil
}

func (s *Server) handleCreateBoard(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	pid, err := parseProjectID(str(args, "projectId"))
	if err != nil {
		return errResult(err), nil
	}
	title := str(args, "title")
	if title == "" {
		return errResult(errors.New("title обязателен")), nil
	}
	bid, err := s.board.CreateBoard(ctx, title, pid)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(fmt.Sprintf(`{"id": %q}`, bid.String())), nil
}

func (s *Server) handleCreateColumn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	bid, err := parseBoardID(str(args, "boardId"))
	if err != nil {
		return errResult(err), nil
	}
	title := str(args, "title")
	if title == "" {
		return errResult(errors.New("title обязателен")), nil
	}
	color := valueobject.ColumnColorDefault
	if c := numInt(args, "color"); c > 0 {
		color = valueobject.ColumnColor(c)
	}
	cid, err := s.board.CreateColumn(ctx, title, bid, color)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(fmt.Sprintf(`{"id": %q}`, cid.String())), nil
}

func (s *Server) handleGetTaskMessages(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	tid, err := parseTaskID(str(args, "taskId"))
	if err != nil {
		return errResult(err), nil
	}
	limit := numInt(args, "limit")
	offset := numInt(args, "offset")
	msgs, paging, err := s.tasks.GetTaskMessages(ctx, tid, limit, offset)
	if err != nil {
		return errResult(err), nil
	}
	data, err := toJSON(map[string]any{"paging": paging, "content": msgs})
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

func (s *Server) handleSendTaskMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	tid, err := parseTaskID(str(args, "taskId"))
	if err != nil {
		return errResult(err), nil
	}
	text := str(args, "text")
	if text == "" {
		return errResult(errors.New("text обязателен")), nil
	}
	// Идентификация отправителя обязательна: несколько агентов работают
	// от одного API-ключа, различать их можно только префиксом.
	// Приоритет (issue #4): явный sender > имя агента из handshake (daemon) >
	// agent_id сервера > ошибка.
	sender := str(args, "sender")
	if sender == "" {
		sender = s.AgentName(ctx)
	}
	if sender == "" {
		sender = s.agentID
	}
	if sender == "" {
		return errResult(errors.New("sender не задан: передайте sender, подключитесь с именем клиента (clientInfo.name) или выставьте YOUGILE_AGENT_ID серверу (идентификация агентов обязательна)")), nil
	}
	id, err := s.tasks.SendTaskMessage(ctx, tid, "["+sender+"] "+text)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(fmt.Sprintf(`{"ok": true, "id": %d}`, id)), nil
}

func (s *Server) handleCreateSticker(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	name := str(args, "name")
	if name == "" {
		return errResult(errors.New("name обязателен")), nil
	}
	cr := sticker.CreateRequest{Title: name, Icon: str(args, "icon")}
	if v := str(args, "boardId"); v != "" {
		bid, err := parseBoardID(v)
		if err != nil {
			return errResult(err), nil
		}
		cr.BoardID = bid
	}
	// states: [{name, color}]
	if raw, ok := args["states"].([]any); ok {
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				return errResult(errors.New("states: ожидается массив {name, color}")), nil
			}
			n := str(m, "name")
			if n == "" {
				return errResult(errors.New("states[].name обязателен")), nil
			}
			opt := sticker.StickerOption{Title: n}
			if c := str(m, "color"); c != "" {
				opt.Color = &c
			}
			cr.Options = append(cr.Options, opt)
		}
	}
	sid, attached, err := s.board.CreateSticker(ctx, cr)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(fmt.Sprintf(`{"id": %q, "attached": %t}`, sid.String(), attached)), nil
}

func (s *Server) handleDeleteBoard(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	bid, err := parseBoardID(str(args, "boardId"))
	if err != nil {
		return errResult(err), nil
	}
	if err := s.board.DeleteBoard(ctx, bid); err != nil {
		return errResult(err), nil
	}
	return textResult(`{"ok": true}`), nil
}

func (s *Server) handleGetStickers(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	bid, err := parseBoardID(str(args, "boardId"))
	if err != nil {
		return errResult(err), nil
	}
	st, err := s.board.ListStickers(ctx, bid)
	if err != nil {
		return errResult(err), nil
	}
	data, err := toJSON(st)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

// ── Хендлеры слоя 2 ──

func (s *Server) handleGetBoardSnapshot(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	bid, err := parseBoardID(str(args, "boardId"))
	if err != nil {
		return errResult(err), nil
	}
	var since *int64
	if d := numInt(args, "since"); d > 0 {
		v := int64(d)
		since = &v
	}
	snap, err := s.board.GetBoardSnapshotFiltered(ctx, bid, since, boardservice.SnapshotFilter{
		IncludeCompleted: boolVal(args, "includeCompleted"),
		IncludeArchived:  boolVal(args, "includeArchived"),
	})
	if err != nil {
		return errResult(err), nil
	}
	f := parseFormat(str(args, "format"))
	if f == formatMarkdown {
		return textResult(renderSnapshotMarkdown(snap)), nil
	}
	data, err := toJSON(snap)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

func (s *Server) handleSummarizeBoard(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	bid, err := parseBoardID(str(args, "boardId"))
	if err != nil {
		return errResult(err), nil
	}
	var since *int64
	if d := numInt(args, "since"); d > 0 {
		v := int64(d)
		since = &v
	}
	sum, err := s.review.Summarize(ctx, bid, since)
	if err != nil {
		return errResult(err), nil
	}
	f := parseFormat(str(args, "format"))
	if f == formatMarkdown {
		return textResult(renderSummary(sum)), nil
	}
	data, err := toJSON(sum)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

func (s *Server) handleAuditBoard(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	bid, err := parseBoardID(str(args, "boardId"))
	if err != nil {
		return errResult(err), nil
	}
	res, err := s.audit.Audit(ctx, auditservice.Params{
		BoardID: bid,
		Rules: auditservice.Rules{
			Overdue:         boolVal(args, "overdue") || boolVal(args, "autoMove"),
			MissingStickers: boolVal(args, "missingStickers"),
			AutoMove:        boolVal(args, "autoMove"),
		},
		DryRun: boolVal(args, "dryRun"),
	})
	if err != nil {
		return errResult(err), nil
	}
	f := parseFormat(str(args, "format"))
	if f == formatMarkdown {
		return textResult(renderAudit(res)), nil
	}
	data, err := toJSON(res)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

func (s *Server) handleTrackGoals(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	bid, err := parseBoardID(str(args, "boardId"))
	if err != nil {
		return errResult(err), nil
	}
	goals, err := s.goal.WeightedKR(ctx, bid)
	if err != nil {
		return errResult(err), nil
	}
	views := make([]goalProgressView, 0, len(goals))
	for _, g := range goals {
		refs := make([]taskRefView, 0, len(g.Tasks))
		for _, tr := range g.Tasks {
			refs = append(refs, taskRefView{TaskID: tr.TaskID.String(), Title: tr.Title, Weight: tr.Weight, Progress: tr.Progress})
		}
		views = append(views, goalProgressView{Goal: g.Goal, WeightedKR: g.WeightedKR, Status: g.Status, Tasks: refs})
	}
	f := parseFormat(str(args, "format"))
	if f == formatMarkdown {
		return textResult(renderGoals(views)), nil
	}
	data, err := toJSON(views)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

func (s *Server) handleBulkMove(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	bid, err := parseBoardID(str(args, "boardId"))
	if err != nil {
		return errResult(err), nil
	}
	target, err := parseColumnID(str(args, "targetColumnId"))
	if err != nil {
		return errResult(err), nil
	}
	var src *valueobject.ColumnID
	if v := str(args, "sourceColumnId"); v != "" {
		c, err := parseColumnID(v)
		if err != nil {
			return errResult(err), nil
		}
		src = &c
	}
	res, err := s.tasks.BulkMove(ctx, taskservice.BulkMoveParams{
		BoardID:        bid,
		SourceColumnID: src,
		TargetColumnID: target,
		DryRun:         boolVal(args, "dryRun"),
	})
	if err != nil {
		return errResult(err), nil
	}
	data, err := toJSON(res)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

func (s *Server) handleBatchStickers(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	bid, err := parseBoardID(str(args, "boardId"))
	if err != nil {
		return errResult(err), nil
	}
	// taskIds и stickers приходят JSON-строками (упрощение для mcp-go v0.58)
	rawTaskIDs := str(args, "taskIds")
	rawStickers := str(args, "stickers")

	var taskIDs []string
	if rawTaskIDs != "" {
		if err := json.Unmarshal([]byte(rawTaskIDs), &taskIDs); err != nil {
			return errResult(fmt.Errorf("taskIds: %w", err)), nil
		}
	}
	var stickersMap map[string]string
	if rawStickers != "" {
		if err := json.Unmarshal([]byte(rawStickers), &stickersMap); err != nil {
			return errResult(fmt.Errorf("stickers: %w", err)), nil
		}
	}

	ids := make([]valueobject.TaskID, 0, len(taskIDs))
	for _, s := range taskIDs {
		t, err := parseTaskID(s)
		if err != nil {
			return errResult(err), nil
		}
		ids = append(ids, t)
	}
	stickers := make(map[valueobject.StickerID]valueobject.StickerValue, len(stickersMap))
	for k, v := range stickersMap {
		sid, err := parseStickerID(k)
		if err != nil {
			return errResult(err), nil
		}
		stickers[sid] = valueobject.StickerValue{Value: v}
	}

	res, err := s.tasks.BatchUpdateStickers(ctx, taskservice.BatchStickersParams{
		BoardID:  bid,
		TaskIDs:  ids,
		Stickers: stickers,
		DryRun:   boolVal(args, "dryRun"),
	})
	if err != nil {
		return errResult(err), nil
	}
	data, err := toJSON(res)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

func (s *Server) handleCompress(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	level := compressionservice.Level(str(args, "level"))
	res, err := s.comp.Compress(ctx, level, compressionservice.TimeRange{
		From: int64(numInt(args, "from")),
		To:   int64(numInt(args, "to")),
	}, nil)
	if err != nil {
		return errResult(err), nil
	}
	data, err := toJSON(res)
	if err != nil {
		return errResult(err), nil
	}
	return textResult(data), nil
}

// ── Хелперы ──

// textResult создаёт текстовый результат.
func textResult(text string) *mcp.CallToolResult {
	return mcp.NewToolResultText(text)
}

// errResult создаёт результат с ошибкой.
func errResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultText("Ошибка: " + err.Error())
}

// renderTasksMarkdown — markdown-список задач.
func renderTasksMarkdown(res taskservice.ListResult) string {
	out := "## Задачи\n\n"
	out += fmt.Sprintf("**Всего**: %d\n\n", len(res.Tasks))
	if len(res.Tasks) == 0 {
		return out + "Пусто\n"
	}
	out += "| Название | Колонка | Статус |\n|---|---|---|\n"
	for _, t := range res.Tasks {
		col := "-"
		if t.ColumnID != nil {
			for _, c := range res.Columns {
				if c.ID == *t.ColumnID {
					col = c.Title
					break
				}
			}
		}
		status := "🔄"
		if t.Completed {
			status = "✅"
		}
		out += fmt.Sprintf("| %s | %s | %s |\n", t.Title, col, status)
	}
	return out
}

// renderSnapshotMarkdown — markdown-снимок доски.
func renderSnapshotMarkdown(snap board.Aggregate) string {
	out := fmt.Sprintf("## %s\n\n", snap.Board.Title)
	out += fmt.Sprintf("**Колонок**: %d, **задач**: %d, **стикеров**: %d\n\n",
		len(snap.Columns), len(snap.Tasks), len(snap.Stickers))
	for _, c := range snap.Columns {
		count := 0
		for _, t := range snap.Tasks {
			if t.ColumnID != nil && *t.ColumnID == c.ID {
				count++
			}
		}
		out += fmt.Sprintf("- **%s**: %d задач\n", c.Title, count)
	}
	return out
}

// handleGetMode — текущий режим доступа.
// В daemon-режиме — режим сессии вызывающего агента, иначе глобальный.
func (s *Server) handleGetMode(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return textResult(fmt.Sprintf(`{"mode": %q}`, string(s.EffectiveMode(ctx)))), nil
}

// handleSetMode — смена режима (read|confirm|yolo).
// stdio: персистится в конфиг (как раньше). daemon: только для сессии агента —
// /yougile-mode yolo у одного не меняет режим другим (issue #4).
func (s *Server) handleSetMode(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	m := config.Mode(str(args, "mode"))
	if !config.ValidMode(string(m)) {
		return errResult(fmt.Errorf("невалидный режим %q: ожидается read|confirm|yolo", m)), nil
	}
	if s.daemon {
		s.sessions.SetMode(SessionIDFromCtx(ctx), m)
		return textResult(fmt.Sprintf(`{"mode": %q, "ok": true, "scope": "session"}`, string(m))), nil
	}
	if !s.SetMode(m) {
		return errResult(errors.New("не удалось сохранить режим в конфиг")), nil
	}
	return textResult(fmt.Sprintf(`{"mode": %q, "ok": true}`, string(m))), nil
}

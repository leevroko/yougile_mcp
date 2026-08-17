// Package mcp — MCP-сервер: инструменты двух слоёв + markdown-презентация.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/yougile-mcp/internal/domain/board"
	"github.com/yougile-mcp/internal/domain/task"
	"github.com/yougile-mcp/internal/domain/valueobject"
	auditservice "github.com/yougile-mcp/internal/service/audit"
	boardservice "github.com/yougile-mcp/internal/service/board"
	compressionservice "github.com/yougile-mcp/internal/service/compression"
	goalservice "github.com/yougile-mcp/internal/service/goal"
	reviewservice "github.com/yougile-mcp/internal/service/review"
	taskservice "github.com/yougile-mcp/internal/service/task"
)

// Server — MCP-сервер YouGile.
type Server struct {
	mcp    *server.MCPServer
	board  boardservice.Service
	tasks  taskservice.Service
	review reviewservice.Service
	audit  auditservice.Service
	goal   goalservice.Service
	comp   compressionservice.Service
}

// New создаёт MCP-сервер с зарегистрированными инструментами.
func New(
	board boardservice.Service,
	tasks taskservice.Service,
	review reviewservice.Service,
	audit auditservice.Service,
	goal goalservice.Service,
	comp compressionservice.Service,
) *Server {
	s := &Server{
		mcp:    server.NewMCPServer("yougile-mcp", "0.1.0", server.WithToolCapabilities(false)),
		board:  board,
		tasks:  tasks,
		review: review,
		audit:  audit,
		goal:   goal,
		comp:   comp,
	}
	s.registerTools()
	return s
}

// MCPServer возвращает внутренний MCP-сервер (для транспортов).
func (s *Server) MCPServer() *server.MCPServer { return s.mcp }

// registerTools регистрирует инструменты двух слоёв.
func (s *Server) registerTools() {
	// ── Слой 1: тонкий CRUD ──
	s.mcp.AddTool(mcp.NewTool("list_projects",
		mcp.WithDescription("Список проектов"),
	), s.handleListProjects)

	s.mcp.AddTool(mcp.NewTool("list_boards",
		mcp.WithDescription("Список досок проекта"),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("ID проекта")),
	), s.handleListBoards)

	s.mcp.AddTool(mcp.NewTool("list_columns",
		mcp.WithDescription("Список колонок доски"),
		mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
	), s.handleListColumns)

	s.mcp.AddTool(mcp.NewTool("list_tasks",
		mcp.WithDescription("Задачи доски/колонки + названия колонок + легенда стикеров"),
		mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
		mcp.WithString("columnId", mcp.Description("ID колонки (опционально)")),
		mcp.WithString("format", mcp.Description("Формат: json|markdown (по умолчанию json)")),
	), s.handleListTasks)

	s.mcp.AddTool(mcp.NewTool("get_task",
		mcp.WithDescription("Задача по ID"),
		mcp.WithString("taskId", mcp.Required(), mcp.Description("ID задачи")),
	), s.handleGetTask)

	s.mcp.AddTool(mcp.NewTool("create_task",
		mcp.WithDescription("Создание задачи"),
		mcp.WithString("title", mcp.Required(), mcp.Description("Название")),
		mcp.WithString("columnId", mcp.Description("ID колонки")),
		mcp.WithString("description", mcp.Description("Описание")),
		mcp.WithNumber("deadline", mcp.Description("Дедлайн (timestamp ms)")),
	), s.handleCreateTask)

	s.mcp.AddTool(mcp.NewTool("update_task",
		mcp.WithDescription("Обновление задачи (перемещение, стикеры, дедлайн)"),
		mcp.WithString("taskId", mcp.Required(), mcp.Description("ID задачи")),
		mcp.WithString("columnId", mcp.Description("Целевая колонка (перемещение)")),
		mcp.WithString("title", mcp.Description("Новое название")),
		mcp.WithString("description", mcp.Description("Новое описание")),
		mcp.WithNumber("deadline", mcp.Description("Дедлайн (timestamp ms)")),
		mcp.WithBoolean("completed", mcp.Description("Выполнена")),
	), s.handleUpdateTask)

	s.mcp.AddTool(mcp.NewTool("get_stickers",
		mcp.WithDescription("Легенда стикеров доски"),
		mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
	), s.handleGetStickers)

	// ── Слой 2: композитные ──
	s.mcp.AddTool(mcp.NewTool("get_board_snapshot",
		mcp.WithDescription("Полное состояние доски: колонки, задачи, стикеры"),
		mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
		mcp.WithNumber("since", mcp.Description("Дельта: timestamp ms, только изменённые")),
		mcp.WithString("format", mcp.Description("Формат: json|markdown")),
	), s.handleGetBoardSnapshot)

	s.mcp.AddTool(mcp.NewTool("summarize_board",
		mcp.WithDescription("Сводка: TL;DR + метрики + группировка + рекомендации"),
		mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
		mcp.WithNumber("since", mcp.Description("Дельта: timestamp ms")),
		mcp.WithString("format", mcp.Description("Формат: json|markdown (по умолчанию markdown)")),
	), s.handleSummarizeBoard)

	s.mcp.AddTool(mcp.NewTool("audit_board",
		mcp.WithDescription("Аудит: просрочка, отсутствие стикеров, авто-перемещение"),
		mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
		mcp.WithBoolean("overdue", mcp.Description("Проверка просрочки")),
		mcp.WithBoolean("missingStickers", mcp.Description("Проверка стикеров")),
		mcp.WithBoolean("autoMove", mcp.Description("Перемещать просроченные в Review")),
		mcp.WithBoolean("dryRun", mcp.Description("Только чтение, без изменений")),
		mcp.WithString("format", mcp.Description("Формат: json|markdown (по умолчанию markdown)")),
	), s.handleAuditBoard)

	s.mcp.AddTool(mcp.NewTool("track_goals",
		mcp.WithDescription("Прогресс целей (weighted KR)"),
		mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
		mcp.WithNumber("since", mcp.Description("Дельта: timestamp ms")),
		mcp.WithString("format", mcp.Description("Формат: json|markdown (по умолчанию markdown)")),
	), s.handleTrackGoals)

	s.mcp.AddTool(mcp.NewTool("bulk_move_tasks",
		mcp.WithDescription("Массовое перемещение задач"),
		mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
		mcp.WithString("sourceColumnId", mcp.Description("Колонка-источник (все задачи)")),
		mcp.WithString("targetColumnId", mcp.Required(), mcp.Description("Целевая колонка")),
		mcp.WithBoolean("dryRun", mcp.Description("Только чтение")),
	), s.handleBulkMove)

	s.mcp.AddTool(mcp.NewTool("batch_update_stickers",
		mcp.WithDescription("Массовое обновление стикеров"),
		mcp.WithString("boardId", mcp.Required(), mcp.Description("ID доски")),
		mcp.WithBoolean("dryRun", mcp.Description("Только чтение")),
	), s.handleBatchStickers)

	s.mcp.AddTool(mcp.NewTool("compress_reviews",
		mcp.WithDescription("Сжатие ревью (daily→weekly→...)"),
		mcp.WithString("level", mcp.Required(), mcp.Description("daily|weekly|monthly|yearly")),
		mcp.WithNumber("from", mcp.Required(), mcp.Description("Начало периода (ms)")),
		mcp.WithNumber("to", mcp.Required(), mcp.Description("Конец периода (ms)")),
	), s.handleCompress)
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
	bid, err := parseBoardID(str(args, "boardId"))
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
	res, err := s.tasks.ListTasks(ctx, bid, cid)
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
	if err := s.tasks.UpdateTask(ctx, tid, ur); err != nil {
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
	snap, err := s.board.GetBoardSnapshot(ctx, bid, since)
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

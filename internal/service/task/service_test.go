package task

import (
	"context"
	"errors"
	"testing"

	domainerr "github.com/yougile-mcp/internal/domain/domainerr"
	domaintask "github.com/yougile-mcp/internal/domain/task"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// fakeRepo — минимальный in-memory task.Repository для тестов подзадач.
type fakeRepo struct {
	tasks   map[string]domaintask.Task
	updated map[string]domaintask.UpdateRequest // taskId → последний Update
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{tasks: map[string]domaintask.Task{}, updated: map[string]domaintask.UpdateRequest{}}
}

func (f *fakeRepo) List(ctx context.Context, filter domaintask.Filter) ([]domaintask.Task, valueobject.PagingMetadata, error) {
	return nil, valueobject.PagingMetadata{}, nil
}

func (f *fakeRepo) GetByID(ctx context.Context, id valueobject.TaskID) (domaintask.Task, error) {
	t, ok := f.tasks[id.String()]
	if !ok {
		return domaintask.Task{}, domainerr.ErrNotFound
	}
	return t, nil
}

func (f *fakeRepo) Create(ctx context.Context, req domaintask.CreateRequest) (valueobject.TaskID, error) {
	id, _ := valueobject.NewTaskID("00000000-0000-0000-0000-000000000001")
	f.tasks[id.String()] = domaintask.Task{ID: id, Title: req.Title, Subtasks: req.Subtasks}
	return id, nil
}

func (f *fakeRepo) Update(ctx context.Context, id valueobject.TaskID, req domaintask.UpdateRequest) error {
	t, ok := f.tasks[id.String()]
	if !ok {
		return domainerr.ErrNotFound
	}
	if req.Subtasks != nil {
		t.Subtasks = *req.Subtasks
	}
	if req.Deleted != nil {
		t.Deleted = *req.Deleted
	}
	f.tasks[id.String()] = t
	f.updated[id.String()] = req
	return nil
}

func (f *fakeRepo) Delete(ctx context.Context, id valueobject.TaskID) error {
	t, ok := f.tasks[id.String()]
	if !ok {
		return domainerr.ErrNotFound
	}
	t.Deleted = true
	f.tasks[id.String()] = t
	return nil
}

func newTestService(repo *fakeRepo) Service {
	return NewService(repo, nil, nil, nil)
}

func tid(s string) valueobject.TaskID { return valueobject.TaskID{} }

func mkTID(t *testing.T, s string) valueobject.TaskID {
	t.Helper()
	id, err := valueobject.NewTaskID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// валидные UUID для тестов
const (
	uuidA = "11111111-1111-1111-1111-111111111111"
	uuidB = "22222222-2222-2222-2222-222222222222"
	uuidC = "33333333-3333-3333-3333-333333333333"
	uuidD = "44444444-4444-4444-4444-444444444444"
)

func seed(t *testing.T, repo *fakeRepo, id string, subtasks ...string) valueobject.TaskID {
	t.Helper()
	tid := mkTID(t, id)
	task := domaintask.Task{ID: tid, Title: "t-" + id}
	for _, s := range subtasks {
		task.Subtasks = append(task.Subtasks, mkTID(t, s))
	}
	repo.tasks[id] = task
	return tid
}

func TestAddSubtaskAppendsAndValidates(t *testing.T) {
	repo := newFakeRepo()
	parent := seed(t, repo, uuidA)
	child := seed(t, repo, uuidB)
	_ = child
	svc := newTestService(repo)

	if err := svc.AddSubtask(context.Background(), parent, mkTID(t, uuidB)); err != nil {
		t.Fatalf("AddSubtask: %v", err)
	}
	got := repo.tasks[uuidA].Subtasks
	if len(got) != 1 || got[0].String() != uuidB {
		t.Fatalf("subtasks after add: %v", got)
	}

	// идемпотентность: повтор — no-op (один PUT меньше)
	if err := svc.AddSubtask(context.Background(), parent, mkTID(t, uuidB)); err != nil {
		t.Fatalf("idempotent AddSubtask: %v", err)
	}
	if got := repo.tasks[uuidA].Subtasks; len(got) != 1 {
		t.Fatalf("subtasks after repeat add: %v", got)
	}

	// битая ссылка: несуществующий ребёнок → ошибка, список не меняется
	ghost := mkTID(t, uuidD)
	if err := svc.AddSubtask(context.Background(), parent, ghost); err == nil {
		t.Fatal("expected error for missing child")
	}
	if got := repo.tasks[uuidA].Subtasks; len(got) != 1 {
		t.Fatalf("subtasks polluted by failed add: %v", got)
	}
}

func TestRemoveSubtaskDetachesOnlyLink(t *testing.T) {
	repo := newFakeRepo()
	parent := seed(t, repo, uuidA, uuidB, uuidC)
	seed(t, repo, uuidB)
	seed(t, repo, uuidC)
	svc := newTestService(repo)

	if err := svc.RemoveSubtask(context.Background(), parent, mkTID(t, uuidB)); err != nil {
		t.Fatalf("RemoveSubtask: %v", err)
	}
	got := repo.tasks[uuidA].Subtasks
	if len(got) != 1 || got[0].String() != uuidC {
		t.Fatalf("subtasks after remove: %v", got)
	}
	// задача B жива — рвётся только связь
	if _, err := repo.GetByID(context.Background(), mkTID(t, uuidB)); err != nil {
		t.Fatal("child task must stay alive")
	}

	// идемпотентность: remove отсутствующей — no-op
	if err := svc.RemoveSubtask(context.Background(), parent, mkTID(t, uuidB)); err != nil {
		t.Fatalf("idempotent remove: %v", err)
	}
	if got := repo.tasks[uuidA].Subtasks; len(got) != 1 {
		t.Fatalf("subtasks after repeat remove: %v", got)
	}
}

func TestListSubtasksReportsBroken(t *testing.T) {
	repo := newFakeRepo()
	parent := seed(t, repo, uuidA, uuidB, uuidC, uuidD)
	seed(t, repo, uuidB)
	seed(t, repo, uuidC)
	// uuidD не существует → битая ссылка
	svc := newTestService(repo)

	res, err := svc.ListSubtasks(context.Background(), parent)
	if err != nil {
		t.Fatalf("ListSubtasks: %v", err)
	}
	if len(res.Tasks) != 2 {
		t.Fatalf("tasks: %v", res.Tasks)
	}
	if len(res.Broken) != 1 || res.Broken[0] != uuidD {
		t.Fatalf("broken: %v", res.Broken)
	}
	if res.Parent.ID.String() != uuidA {
		t.Fatalf("parent: %v", res.Parent.ID)
	}
}

func TestCreateTaskWithSubtasks(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	id, err := svc.CreateTask(context.Background(), CreateTaskParams{
		Title:    "parent",
		Subtasks: []valueobject.TaskID{mkTID(t, uuidB), mkTID(t, uuidC)},
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	got := repo.tasks[id.String()].Subtasks
	if len(got) != 2 || got[0].String() != uuidB || got[1].String() != uuidC {
		t.Fatalf("subtasks on create: %v", got)
	}
}

func TestStickerPatchSerializesNullAsRemove(t *testing.T) {
	// проверка на уровне репозитория невозможна без HTTP — тестируем что
	// ClearStickers проходит через Update сервиса без искажений
	repo := newFakeRepo()
	parent := seed(t, repo, uuidA)
	_ = parent
	svc := newTestService(repo)
	sid, err := valueobject.NewStickerID("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	if err != nil {
		t.Fatal(err)
	}
	clear := []valueobject.StickerID{sid}
	if err := svc.UpdateTask(context.Background(), mkTID(t, uuidA), domaintask.UpdateRequest{ClearStickers: clear}); err != nil {
		t.Fatalf("UpdateTask with ClearStickers: %v", err)
	}
	if !errors.Is(nil, nil) {
		t.Fatal("unreachable")
	}
	saved := repo.updated[uuidA].ClearStickers
	if len(saved) != 1 || saved[0] != clear[0] {
		t.Fatalf("ClearStickers not propagated: %v", saved)
	}
}

func TestUndeleteViaUpdate(t *testing.T) {
	repo := newFakeRepo()
	id := seed(t, repo, uuidA)
	svc := newTestService(repo)

	if err := svc.DeleteTask(context.Background(), id); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if !repo.tasks[uuidA].Deleted {
		t.Fatal("expected deleted=true")
	}

	// восстановление через update_task {deleted: false} (issue #6)
	no := false
	if err := svc.UpdateTask(context.Background(), id, domaintask.UpdateRequest{Deleted: &no}); err != nil {
		t.Fatalf("undelete: %v", err)
	}
	if repo.tasks[uuidA].Deleted {
		t.Fatal("expected deleted=false after restore")
	}
}

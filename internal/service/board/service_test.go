package board

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yougile-mcp/internal/domain/board"
	"github.com/yougile-mcp/internal/domain/column"
	"github.com/yougile-mcp/internal/domain/sticker"
	"github.com/yougile-mcp/internal/domain/task"
	"github.com/yougile-mcp/internal/domain/valueobject"
)

// ── Фейковые репозитории (embed интерфейса: неиспользуемые методы паникуют) ──

type fakeBoards struct {
	board.Repository
	b board.Board
}

func (f *fakeBoards) GetByID(_ context.Context, id valueobject.BoardID) (board.Board, error) {
	if id != f.b.ID {
		return board.Board{}, errors.New("not found")
	}
	return f.b, nil
}

type fakeColumns struct {
	column.Repository
	cols []column.Column
}

func (f *fakeColumns) List(_ context.Context, _ valueobject.BoardID) ([]column.Column, valueobject.PagingMetadata, error) {
	return f.cols, valueobject.PagingMetadata{Count: len(f.cols)}, nil
}

type fakeTasks struct {
	task.Repository
	mu         sync.Mutex
	pages      map[valueobject.ColumnID][][]task.Task // колонка → страницы по 100
	delay      time.Duration                          // задержка на каждый List-вызов
	inFlight   int32
	maxInFligh int32
}

func (f *fakeTasks) List(ctx context.Context, filter task.Filter) ([]task.Task, valueobject.PagingMetadata, error) {
	cur := atomic.AddInt32(&f.inFlight, 1)
	for {
		max := atomic.LoadInt32(&f.maxInFligh)
		if cur <= max || atomic.CompareAndSwapInt32(&f.maxInFligh, max, cur) {
			break
		}
	}
	defer atomic.AddInt32(&f.inFlight, -1)

	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, valueobject.PagingMetadata{}, ctx.Err()
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	pages, ok := f.pages[*filter.ColumnID]
	if !ok {
		return nil, valueobject.PagingMetadata{}, nil
	}
	offset := filter.Offset / 100
	if offset >= len(pages) {
		return nil, valueobject.PagingMetadata{}, nil
	}
	page := pages[offset]
	next := offset+1 < len(pages)
	paging := valueobject.PagingMetadata{Count: len(page), Limit: 100, Offset: offset * 100, Next: next}
	return page, paging, nil
}

type fakeStickers struct {
	sticker.Repository
}

func (f *fakeStickers) List(_ context.Context, _ valueobject.BoardID) ([]sticker.Sticker, error) {
	return nil, nil
}

// ── Хелперы ─────────────────────────────────────────────────────────────

func mkBoardID(i int) valueobject.BoardID {
	id, _ := valueobject.NewBoardID(fmt.Sprintf("00000000-0000-0000-0000-%012d", i))
	return id
}

func mkColID(i int) valueobject.ColumnID {
	id, _ := valueobject.NewColumnID(fmt.Sprintf("10000000-0000-0000-0000-%012d", i))
	return id
}

func mkTaskID(i int) valueobject.TaskID {
	id, _ := valueobject.NewTaskID(fmt.Sprintf("20000000-0000-0000-0000-%012d", i))
	return id
}

func newTestService(ft *fakeTasks, cols []column.Column) Service {
	return NewService(
		nil, // projects — не используется в snapshot
		&fakeBoards{b: board.Board{ID: mkBoardID(1)}},
		&fakeColumns{cols: cols},
		&fakeStickers{},
		ft,
	)
}

func mkCols(n int) []column.Column {
	cols := make([]column.Column, n)
	for i := range cols {
		cols[i] = column.Column{ID: mkColID(i), Title: fmt.Sprintf("col-%d", i), BoardID: mkBoardID(1)}
	}
	return cols
}

// mkTasks — n задач с ID, кодирующим порядковый номер (для проверки порядка).
func mkTasks(colIdx, from, n int) []task.Task {
	out := make([]task.Task, 0, n)
	for j := 0; j < n; j++ {
		out = append(out, task.Task{ID: mkTaskID(colIdx*1000 + from + j), ColumnID: ptrCol(mkColID(colIdx))})
	}
	return out
}

func ptrCol(id valueobject.ColumnID) *valueobject.ColumnID { return &id }

// ── Тесты ───────────────────────────────────────────────────────────────

// Полный обход: все колонки, все страницы; порядок колонок сохранён;
// фильтры (deleted/completed/since) работают.
func TestSnapshotCollectsAllTasks(t *testing.T) {
	cols := mkCols(3)
	pages := map[valueobject.ColumnID][][]task.Task{
		mkColID(0): {mkTasks(0, 0, 100), mkTasks(0, 100, 100), mkTasks(0, 200, 50)}, // 3 страницы
		mkColID(1): {mkTasks(1, 0, 7)},
		mkColID(2): {mkTasks(2, 0, 0)}, // пустая колонка
	}
	// Внутри первой страницы — задачи на фильтрацию
	pages[mkColID(0)][0][0].Deleted = true
	pages[mkColID(0)][0][1].Completed = true
	pages[mkColID(1)][0][0].Archived = true

	svc := newTestService(&fakeTasks{pages: pages}, cols)
	snap, err := svc.GetBoardSnapshot(context.Background(), mkBoardID(1), nil)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	want := 250 - 2 + 7 - 1 // минус deleted+completed из col-0, минус archived из col-1
	if len(snap.Tasks) != want {
		t.Fatalf("задач = %d, хотим %d", len(snap.Tasks), want)
	}
	if len(snap.Columns) != 3 {
		t.Fatalf("колонок = %d, хотим 3", len(snap.Columns))
	}
	// Порядок: сначала все задачи col-0 (пропущены 2), потом col-1 (пропущена 1)
	first := snap.Tasks[0].ID
	if first != mkTaskID(2) { // col-0, index 2 (0=deleted, 1=completed)
		t.Fatalf("первая задача = %v, хотим col-0 index 2", first)
	}
	last := snap.Tasks[len(snap.Tasks)-1].ID
	if last != mkTaskID(1006) { // col-1, последняя из 7 минус archived-первая
		t.Fatalf("последняя задача = %v, хотим col-1 index 6", last)
	}
}

// Параллелизм: колонки фетчатся конкурентно, но не более snapshotFetchConcurrency.
func TestSnapshotBoundedConcurrency(t *testing.T) {
	cols := mkCols(10)
	pages := map[valueobject.ColumnID][][]task.Task{}
	for i := range cols {
		pages[mkColID(i)] = [][]task.Task{mkTasks(i, 0, 5)}
	}
	ft := &fakeTasks{pages: pages, delay: 100 * time.Millisecond}
	svc := newTestService(ft, cols)

	start := time.Now()
	if _, err := svc.GetBoardSnapshot(context.Background(), mkBoardID(1), nil); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	elapsed := time.Since(start)

	if got := atomic.LoadInt32(&ft.maxInFligh); got > snapshotFetchConcurrency {
		t.Fatalf("max in-flight = %d, лимит %d", got, snapshotFetchConcurrency)
	}
	// Последовательно 10×100мс = 1с; параллельно волнами по 4 → ~300мс.
	if elapsed >= 900*time.Millisecond {
		t.Fatalf("слишком медленно: %v (параллелизм не работает?)", elapsed)
	}
	t.Logf("elapsed=%v maxInFlight=%d", elapsed, ft.maxInFligh)
}

// Дедлайн: медленный API → контекст отменяется, snapshot возвращает ошибку.
func TestSnapshotDeadline(t *testing.T) {
	oldTimeout := snapshotTimeout
	snapshotTimeout = 150 * time.Millisecond
	defer func() { snapshotTimeout = oldTimeout }()

	cols := mkCols(2)
	pages := map[valueobject.ColumnID][][]task.Task{
		mkColID(0): {mkTasks(0, 0, 5)},
		mkColID(1): {mkTasks(1, 0, 5)},
	}
	ft := &fakeTasks{pages: pages, delay: 500 * time.Millisecond}
	svc := newTestService(ft, cols)

	_, err := svc.GetBoardSnapshot(context.Background(), mkBoardID(1), nil)
	if err == nil {
		t.Fatal("ожидали ошибку (deadline), получили nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ошибка = %v, хотим context.DeadlineExceeded", err)
	}
}

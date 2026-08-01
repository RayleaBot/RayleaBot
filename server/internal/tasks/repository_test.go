package tasks

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestSQLiteRepository_SaveAndLoad(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	ctx := context.Background()
	startedAt := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	finishedAt := time.Date(2026, 3, 20, 10, 1, 0, 0, time.UTC)

	snapshot := Snapshot{
		TaskID:     "task_abc123",
		TaskType:   "plugin.install",
		Status:     StatusSucceeded,
		Progress:   100,
		Summary:    "安装完成",
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
		Result:     &ResultSummary{Summary: "installed hello-go"},
	}

	if err := repo.SaveTask(ctx, snapshot); err != nil {
		t.Fatalf("save task: %v", err)
	}

	loaded, err := repo.LoadTasks(ctx)
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d tasks, want 1", len(loaded))
	}

	got := loaded[0]
	if got.TaskID != snapshot.TaskID {
		t.Errorf("TaskID = %q, want %q", got.TaskID, snapshot.TaskID)
	}
	if got.TaskType != snapshot.TaskType {
		t.Errorf("TaskType = %q, want %q", got.TaskType, snapshot.TaskType)
	}
	if got.Status != snapshot.Status {
		t.Errorf("Status = %q, want %q", got.Status, snapshot.Status)
	}
	if got.Progress != 100 {
		t.Errorf("Progress = %d, want 100", got.Progress)
	}
	if got.Result == nil || got.Result.Summary != "installed hello-go" {
		t.Errorf("Result = %+v, want summary 'installed hello-go'", got.Result)
	}
	if got.StartedAt == nil || !got.StartedAt.Equal(startedAt) {
		t.Errorf("StartedAt = %v, want %v", got.StartedAt, startedAt)
	}
	if got.FinishedAt == nil || !got.FinishedAt.Equal(finishedAt) {
		t.Errorf("FinishedAt = %v, want %v", got.FinishedAt, finishedAt)
	}
}

func TestSQLiteRepository_Upsert(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	ctx := context.Background()
	snapshot := Snapshot{
		TaskID:   "task_upsert1",
		TaskType: "backup.create",
		Status:   StatusPending,
		Summary:  "pending backup",
	}

	if err := repo.SaveTask(ctx, snapshot); err != nil {
		t.Fatalf("save initial: %v", err)
	}

	// Update status.
	snapshot.Status = StatusRunning
	snapshot.Progress = 50
	snapshot.Summary = "halfway"
	if err := repo.SaveTask(ctx, snapshot); err != nil {
		t.Fatalf("save update: %v", err)
	}

	loaded, err := repo.LoadTasks(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d tasks, want 1", len(loaded))
	}
	if loaded[0].Status != StatusRunning {
		t.Errorf("Status = %q, want running", loaded[0].Status)
	}
	if loaded[0].Progress != 50 {
		t.Errorf("Progress = %d, want 50", loaded[0].Progress)
	}
}

func TestSQLiteRepository_DoesNotDowngradeTerminalSnapshot(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	ctx := context.Background()
	startedAt := time.Date(2026, 4, 2, 7, 41, 19, 0, time.UTC)
	finishedAt := time.Date(2026, 4, 2, 7, 41, 21, 0, time.UTC)
	taskID := "task_runtime_bootstrap_0001"

	succeeded := Snapshot{
		TaskID:     taskID,
		TaskType:   "runtime.bootstrap",
		Status:     StatusSucceeded,
		Progress:   100,
		Summary:    "运行环境准备完成",
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
		Result: &ResultSummary{
			Summary: "运行环境准备完成",
			Details: map[string]any{
				"resources": []any{
					map[string]any{
						"resource": "chromium",
						"status":   "ready",
					},
				},
			},
		},
	}
	if err := repo.SaveTask(ctx, succeeded); err != nil {
		t.Fatalf("save succeeded snapshot: %v", err)
	}

	staleRunning := Snapshot{
		TaskID:    taskID,
		TaskType:  "runtime.bootstrap",
		Status:    StatusRunning,
		Progress:  90,
		Summary:   "准备运行环境",
		StartedAt: &startedAt,
	}
	if err := repo.SaveTask(ctx, staleRunning); err != nil {
		t.Fatalf("save stale running snapshot: %v", err)
	}

	loaded, err := repo.LoadTasks(ctx)
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d tasks, want 1", len(loaded))
	}

	got := loaded[0]
	if got.Status != StatusSucceeded {
		t.Fatalf("Status = %q, want %q", got.Status, StatusSucceeded)
	}
	if got.Progress != 100 {
		t.Fatalf("Progress = %d, want 100", got.Progress)
	}
	if got.FinishedAt == nil || !got.FinishedAt.Equal(finishedAt) {
		t.Fatalf("FinishedAt = %v, want %v", got.FinishedAt, finishedAt)
	}
	resources, ok := got.Result.Details["resources"].([]any)
	if got.Result == nil || !ok || len(resources) != 1 {
		t.Fatalf("Result = %+v, want persisted runtime bootstrap details", got.Result)
	}
}

func TestSQLiteRepository_Delete(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	ctx := context.Background()
	snapshot := Snapshot{
		TaskID:   "task_del1",
		TaskType: "plugin.reload",
		Status:   StatusSucceeded,
		Summary:  "done",
	}

	if err := repo.SaveTask(ctx, snapshot); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := repo.DeleteTask(ctx, "task_del1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	loaded, err := repo.LoadTasks(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("loaded %d tasks, want 0", len(loaded))
	}
}

func TestSQLiteRepository_ErrorSummary(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	ctx := context.Background()
	snapshot := Snapshot{
		TaskID:   "task_err1",
		TaskType: "plugin.install",
		Status:   StatusFailed,
		Summary:  "install failed",
		Error:    &ErrorSummary{Code: "plugin.install_failed", Message: "manifest invalid"},
	}

	if err := repo.SaveTask(ctx, snapshot); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := repo.LoadTasks(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d tasks, want 1", len(loaded))
	}
	if loaded[0].Error == nil || loaded[0].Error.Code != "plugin.install_failed" {
		t.Errorf("Error = %+v, want code 'plugin.install_failed'", loaded[0].Error)
	}
}

func TestSQLiteRepository_NilStore(t *testing.T) {
	t.Parallel()
	_, err := NewSQLiteRepository(nil)
	if err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestRegistry_HydrateFromRepository(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	ctx := context.Background()

	// Seed two tasks directly into the database.
	for _, s := range []Snapshot{
		{TaskID: "task_h1", TaskType: "plugin.install", Status: StatusSucceeded, Summary: "done"},
		{TaskID: "task_h2", TaskType: "backup.create", Status: StatusFailed, Summary: "failed"},
	} {
		if err := repo.SaveTask(ctx, s); err != nil {
			t.Fatalf("seed task %s: %v", s.TaskID, err)
		}
	}

	// Create a fresh registry and hydrate.
	registry := NewRegistry()
	registry.SetRepository(repo)
	if err := registry.Hydrate(ctx); err != nil {
		t.Fatalf("hydrate: %v", err)
	}

	items := registry.List()
	if len(items) != 2 {
		t.Fatalf("list returned %d items, want 2", len(items))
	}

	snap, ok := registry.Get("task_h1")
	if !ok {
		t.Fatal("task_h1 not found after hydrate")
	}
	if snap.Status != StatusSucceeded {
		t.Errorf("task_h1 status = %q, want succeeded", snap.Status)
	}
}

func TestRegistryHydrateInterruptsInProgressTasks(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	ctx := context.Background()
	for _, snapshot := range []Snapshot{
		{TaskID: "task_pending", TaskType: "plugin.install", Status: StatusPending, Summary: "pending"},
		{TaskID: "task_running", TaskType: "backup.create", Status: StatusRunning, Summary: "running"},
		{TaskID: "task_done", TaskType: "backup.create", Status: StatusSucceeded, Summary: "done"},
	} {
		if err := repo.SaveTask(ctx, snapshot); err != nil {
			t.Fatalf("seed %s: %v", snapshot.TaskID, err)
		}
	}

	registry := NewRegistry()
	registry.SetRepository(repo)
	if err := registry.Hydrate(ctx); err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	defer registry.Close()

	for _, taskID := range []string{"task_pending", "task_running"} {
		snapshot, ok := registry.Get(taskID)
		if !ok || snapshot.Status != StatusInterrupted || snapshot.FinishedAt == nil {
			t.Fatalf("%s was not reconciled: %#v", taskID, snapshot)
		}
	}
	if snapshot, _ := registry.Get("task_done"); snapshot.Status != StatusSucceeded {
		t.Fatalf("terminal task changed during hydrate: %#v", snapshot)
	}

	persisted, err := repo.LoadTasks(ctx)
	if err != nil {
		t.Fatalf("reload reconciled tasks: %v", err)
	}
	for _, snapshot := range persisted {
		if snapshot.TaskID != "task_done" && snapshot.Status != StatusInterrupted {
			t.Fatalf("persisted task was not interrupted: %#v", snapshot)
		}
	}
}

type recordingRepository struct {
	mu          sync.Mutex
	saves       []Snapshot
	firstSave   chan struct{}
	releaseSave chan struct{}
}

func (r *recordingRepository) SaveTask(_ context.Context, snapshot Snapshot) error {
	if r.firstSave != nil {
		close(r.firstSave)
		<-r.releaseSave
		r.firstSave = nil
	}
	r.mu.Lock()
	r.saves = append(r.saves, cloneSnapshot(snapshot))
	r.mu.Unlock()
	return nil
}

func (*recordingRepository) LoadTasks(context.Context) ([]Snapshot, error) { return nil, nil }
func (*recordingRepository) DeleteTask(context.Context, string) error      { return nil }
func (*recordingRepository) InterruptInProgressTasks(context.Context, time.Time) error {
	return nil
}

func (r *recordingRepository) snapshots() []Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Snapshot(nil), r.saves...)
}

func TestRegistryCreateWaitsForFirstPersistence(t *testing.T) {
	repo := &recordingRepository{
		firstSave:   make(chan struct{}),
		releaseSave: make(chan struct{}),
	}
	registry := NewRegistry()
	registry.SetRepository(repo)
	defer registry.Close()

	returned := make(chan error, 1)
	go func() {
		_, err := registry.Create("backup.create", "durable create")
		returned <- err
	}()

	<-repo.firstSave
	select {
	case err := <-returned:
		t.Fatalf("Create returned before persistence completed: %v", err)
	default:
	}
	close(repo.releaseSave)
	if err := <-returned; err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestRegistryCloseDrainsUpdatesInOrder(t *testing.T) {
	repo := &recordingRepository{}
	registry := NewRegistry()
	registry.SetRepository(repo)

	taskID, err := registry.Create("runtime.bootstrap", "pending")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for progress := 1; progress <= 3; progress++ {
		if _, ok := registry.Update(taskID, Update{Progress: &progress}); !ok {
			t.Fatalf("Update(%d) failed", progress)
		}
	}
	if err := registry.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	saves := repo.snapshots()
	if len(saves) != 4 {
		t.Fatalf("saves = %d, want 4", len(saves))
	}
	for index, snapshot := range saves {
		if snapshot.Progress != index {
			t.Fatalf("save[%d].Progress = %d, want %d", index, snapshot.Progress, index)
		}
	}
}

func TestSQLiteRepository_EmptyDB(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "empty.db")
	_ = os.Remove(dbPath)

	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	loaded, err := repo.LoadTasks(context.Background())
	if err != nil {
		t.Fatalf("load from empty: %v", err)
	}
	if loaded != nil && len(loaded) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(loaded))
	}
}

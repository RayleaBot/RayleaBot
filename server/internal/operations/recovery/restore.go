package recovery

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/fsguard"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/runtimepaths"
)

const (
	maxRestoreEntries     = 100_000
	maxRestoreFileBytes   = 2 << 30
	maxRestoreBytes       = 8 << 30
	maxRestoreRatio       = 1000
	restoredDatabaseEntry = "data/rayleabot.db"
)

var errRestoreResourceLimit = errors.New("restore archive resource limit exceeded")

type RestoreOptions struct {
	ConfigPath  string
	ArchivePath string
}

type RestoreResult struct {
	RestoredFiles     int
	DatabasePath      string
	DatabaseRelocated bool
	Committed         bool
	Summary           CompatibilitySummary
}

// RestoreError reports whether committed files or recovery material need
// attention without including untrusted archive content in its message.
type RestoreError struct {
	Stage             string
	RecoveryDirectory string
	cause             error
}

func (e *RestoreError) Error() string {
	switch e.Stage {
	case "rollback":
		return "恢复失败且回滚未完成，已保留恢复工作目录"
	case "cleanup":
		return "数据已恢复，但恢复工作目录清理失败"
	default:
		return "恢复失败（" + e.Stage + "），目标文件未改变或已回滚"
	}
}

func (e *RestoreError) Unwrap() error { return e.cause }

type restoreDeps struct {
	copy      func(context.Context, *zip.File, *os.File) error
	rename    func(*os.Root, string, string) error
	remove    func(*os.Root, string) error
	removeAll func(*os.Root, string) error
}

func Restore(ctx context.Context, options RestoreOptions) (RestoreResult, error) {
	return restoreWithDeps(ctx, options, restoreDeps{})
}

func restoreWithDeps(ctx context.Context, options RestoreOptions, deps restoreDeps) (result RestoreResult, retErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	deps = deps.withDefaults()
	repoRoot, configRelative, err := resolveRestoreTarget(options)
	if err != nil {
		return result, err
	}
	archive, err := zip.OpenReader(options.ArchivePath)
	if err != nil {
		return result, &RestoreError{Stage: "archive", cause: err}
	}
	defer func() { retErr = errors.Join(retErr, archive.Close()) }()
	manifest, entries, err := inspectRestoreArchive(archive.File)
	if err != nil {
		return result, &RestoreError{Stage: "preflight", cause: err}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	workspace, err := newRestoreWorkspace(repoRoot, deps)
	if err != nil {
		return result, err
	}
	defer func() { retErr = workspace.close(result.Committed, retErr) }()
	if err := workspace.extract(ctx, entries); err != nil {
		return result, err
	}
	databaseRelative, relocated, err := workspace.prepareConfig(manifest)
	if err != nil {
		return result, err
	}
	result.DatabasePath = filepath.Join(repoRoot, filepath.FromSlash(databaseRelative))
	result.DatabaseRelocated = relocated
	hasDatabase := manifest.DBSchemaVersion != "absent"
	if err := workspace.verifyDatabase(ctx, manifest); err != nil {
		return result, err
	}
	result.Summary = EvaluateRestore(manifest, repoRoot)
	if result.Summary.Status == "blocked" {
		return result, errors.New("restore manifest failed preflight")
	}
	if err := workspace.saveSummary(result.Summary); err != nil {
		return result, err
	}
	writes, err := workspace.planWrites(entries, configRelative, databaseRelative, hasDatabase)
	if err != nil {
		return result, err
	}
	lock, err := lockRestoreDatabase(workspace.root, databaseRelative, hasDatabase)
	if err != nil {
		return result, err
	}
	if lock != nil {
		defer func() { retErr = errors.Join(retErr, lock.Close()) }()
	}
	if err := workspace.applyWrites(ctx, writes); err != nil {
		return result, err
	}
	result.Committed = true
	for _, write := range writes {
		if write.staged != "" && write.target != filepath.FromSlash(RecoverySummaryPath) {
			result.RestoredFiles++
		}
	}
	return result, nil
}

func (deps restoreDeps) withDefaults() restoreDeps {
	if deps.copy == nil {
		deps.copy = copyRestoreEntry
	}
	if deps.rename == nil {
		deps.rename = (*os.Root).Rename
	}
	if deps.remove == nil {
		deps.remove = (*os.Root).Remove
	}
	if deps.removeAll == nil {
		deps.removeAll = (*os.Root).RemoveAll
	}
	return deps
}

func resolveRestoreTarget(options RestoreOptions) (string, string, error) {
	if options.ConfigPath == "" || options.ArchivePath == "" {
		return "", "", errors.New("restore configuration and archive paths are required")
	}
	configPath, err := filepath.Abs(options.ConfigPath)
	if err != nil {
		return "", "", err
	}
	repoRoot := runtimepaths.RootFromConfigPath(configPath)
	relative, err := filepath.Rel(repoRoot, configPath)
	if err != nil || !fsguard.WithinRoot(repoRoot, configPath) {
		return "", "", errors.New("restore configuration must be inside target root")
	}
	return repoRoot, relative, nil
}

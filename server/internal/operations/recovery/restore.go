package recovery

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/runtimepaths"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/fsguard"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/filelock"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
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

type restoreWrite struct {
	target    string
	staged    string
	previous  string
	backedUp  bool
	installed bool
}

func Restore(ctx context.Context, options RestoreOptions) (RestoreResult, error) {
	return restoreWithDeps(ctx, options, restoreDeps{})
}

func restoreWithDeps(ctx context.Context, options RestoreOptions, deps restoreDeps) (result RestoreResult, retErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
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
	if options.ConfigPath == "" || options.ArchivePath == "" {
		return result, errors.New("restore configuration and archive paths are required")
	}
	configPath, err := filepath.Abs(options.ConfigPath)
	if err != nil {
		return result, err
	}
	repoRoot := runtimepaths.RootFromConfigPath(configPath)
	configRelative, err := filepath.Rel(repoRoot, configPath)
	if err != nil || !fsguard.WithinRoot(repoRoot, configPath) {
		return result, errors.New("restore configuration must be inside target root")
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
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		return result, err
	}
	if info, err := os.Lstat(repoRoot); err != nil || info.Mode()&os.ModeSymlink != 0 {
		return result, errors.New("restore root must be a real directory")
	}
	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return result, err
	}
	defer func() { retErr = errors.Join(retErr, root.Close()) }()
	work := ".restore-" + rand.Text()
	if err := root.Mkdir(work, 0o700); err != nil {
		return result, err
	}
	keepWork := false
	defer func() {
		if !keepWork {
			if cleanupErr := deps.removeAll(root, work); cleanupErr != nil {
				stage := "preflight cleanup"
				if result.Committed {
					stage = "cleanup"
				}
				retErr = &RestoreError{Stage: stage, RecoveryDirectory: filepath.Join(repoRoot, work), cause: errors.Join(retErr, cleanupErr)}
			}
		}
	}()
	for name, entry := range entries {
		if name == "backup-manifest.json" || entry.FileInfo().IsDir() {
			continue
		}
		if err := root.MkdirAll(filepath.Join(work, "incoming", filepath.Dir(filepath.FromSlash(name))), 0o700); err != nil {
			return result, err
		}
		file, openErr := root.OpenFile(filepath.Join(work, "incoming", filepath.FromSlash(name)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, entry.Mode().Perm())
		if openErr != nil {
			return result, openErr
		}
		copyErr := deps.copy(ctx, entry, file)
		copyErr = errors.Join(copyErr, file.Close())
		if copyErr != nil {
			return result, &RestoreError{Stage: "extract", cause: copyErr}
		}
	}
	stagedConfig := filepath.Join(work, "incoming", "config", "user.yaml")
	document, err := config.LoadDocument(filepath.Join(repoRoot, stagedConfig), "")
	if err != nil {
		return result, &RestoreError{Stage: "config", cause: err}
	}
	if document["schema_version"] != manifest.ConfigSchemaVersion {
		return result, errors.New("archived configuration version does not match manifest")
	}
	databaseDocument, ok := document["database"].(map[string]any)
	if !ok {
		return result, errors.New("archived configuration has no database definition")
	}
	sourceDatabase, _ := databaseDocument["path"].(string)
	databaseRelative, relocated, err := portableDatabasePath(sourceDatabase)
	if err != nil {
		return result, &RestoreError{Stage: "database path", cause: err}
	}
	databaseDocument["path"] = databaseRelative
	configPayload, err := config.MarshalDocument(document)
	if err != nil {
		return result, err
	}
	if err := root.WriteFile(stagedConfig, configPayload, 0o600); err != nil {
		return result, err
	}
	result.DatabasePath = filepath.Join(repoRoot, filepath.FromSlash(databaseRelative))
	result.DatabaseRelocated = relocated
	hasDatabase := manifest.DBSchemaVersion != "absent"
	if hasDatabase {
		stagedDatabase := filepath.Join(repoRoot, work, "incoming", filepath.FromSlash(restoredDatabaseEntry))
		if err := storage.QuickCheckPath(ctx, stagedDatabase); err != nil {
			return result, &RestoreError{Stage: "database", cause: err}
		}
		version, err := storage.ReadSchemaVersion(ctx, stagedDatabase)
		if err != nil || version != manifest.DBSchemaVersion {
			return result, &RestoreError{Stage: "database metadata", cause: errors.Join(err, errors.New("snapshot version differs from manifest"))}
		}
	}
	result.Summary = EvaluateRestore(manifest, repoRoot)
	if result.Summary.Status == "blocked" {
		return result, errors.New("restore manifest failed preflight")
	}
	summaryPayload, err := json.MarshalIndent(result.Summary, "", "  ")
	if err != nil {
		return result, err
	}
	if err := root.WriteFile(filepath.Join(work, "summary.json"), append(summaryPayload, '\n'), 0o600); err != nil {
		return result, err
	}
	writes := make([]restoreWrite, 0, len(entries)+4)
	for name, entry := range entries {
		if name == "backup-manifest.json" || entry.FileInfo().IsDir() {
			continue
		}
		target := name
		if name == "config/user.yaml" {
			target = filepath.ToSlash(configRelative)
		}
		if name == restoredDatabaseEntry && hasDatabase {
			target = databaseRelative
		}
		writes = append(writes, restoreWrite{target: filepath.FromSlash(target), staged: filepath.Join(work, "incoming", filepath.FromSlash(name))})
	}
	if hasDatabase {
		for _, suffix := range []string{"-wal", "-shm", "-journal"} {
			writes = append(writes, restoreWrite{target: filepath.FromSlash(databaseRelative) + suffix})
		}
	}
	writes = append(writes, restoreWrite{target: filepath.FromSlash(RecoverySummaryPath), staged: filepath.Join(work, "summary.json")})
	sort.Slice(writes, func(i, j int) bool { return writes[i].target < writes[j].target })
	seenTargets := make(map[string]bool, len(writes))
	for _, write := range writes {
		key := strings.ToLower(filepath.ToSlash(write.target))
		if seenTargets[key] {
			return result, errors.New("restored paths collide after database relocation")
		}
		seenTargets[key] = true
		if err := checkRestoreTarget(root, write.target); err != nil {
			return result, &RestoreError{Stage: "target", cause: err}
		}
	}
	for target := range seenTargets {
		for parent := path.Dir(target); parent != "."; parent = path.Dir(parent) {
			if seenTargets[parent] {
				return result, errors.New("restored file paths collide with a parent directory")
			}
		}
	}
	// Lock the actual destination database even when another configuration file
	// in the same runtime root points at it. The CLI owns its configuration lock.
	if hasDatabase {
		lockPath := filepath.FromSlash(databaseRelative) + ".lock"
		if err := checkRestoreTarget(root, lockPath); err != nil {
			return result, err
		}
		if err := root.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
			return result, err
		}
		file, err := root.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return result, err
		}
		lock, err := filelock.AcquireFile(file)
		if err != nil {
			return result, err
		}
		defer func() { retErr = errors.Join(retErr, lock.Close()) }()
	}
	if err := root.Mkdir(filepath.Join(work, "previous"), 0o700); err != nil {
		return result, err
	}
	createdDirs := []string{}
	applied := 0
	for index := range writes {
		write := &writes[index]
		write.previous = filepath.Join(work, "previous", write.target)
		commitErr := ctx.Err()
		if commitErr == nil {
			commitErr = checkRestoreTarget(root, write.target)
		}
		if commitErr == nil {
			commitErr = createRestoreParents(root, filepath.Dir(write.target), &createdDirs)
		}
		if commitErr == nil {
			commitErr = root.MkdirAll(filepath.Dir(write.previous), 0o700)
		}
		if commitErr == nil {
			if _, statErr := root.Lstat(write.target); statErr == nil {
				commitErr = deps.rename(root, write.target, write.previous)
				write.backedUp = commitErr == nil
			} else if !errors.Is(statErr, os.ErrNotExist) {
				commitErr = statErr
			}
		}
		applied = index + 1
		if commitErr == nil && write.staged != "" {
			commitErr = deps.rename(root, write.staged, write.target)
			write.installed = commitErr == nil
		}
		if commitErr == nil {
			commitErr = ctx.Err()
		}
		if commitErr != nil {
			rollbackErr := rollbackRestore(root, writes[:applied], createdDirs, deps)
			stage := "write"
			if rollbackErr != nil {
				keepWork, stage = true, "rollback"
			}
			failure := &RestoreError{Stage: stage, cause: errors.Join(commitErr, rollbackErr)}
			if keepWork {
				failure.RecoveryDirectory = filepath.Join(repoRoot, work)
			}
			return result, failure
		}
	}
	result.Committed = true
	for _, write := range writes {
		if write.staged != "" && write.target != filepath.FromSlash(RecoverySummaryPath) {
			result.RestoredFiles++
		}
	}
	return result, nil
}

func portableDatabasePath(value string) (string, bool, error) {
	slash := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if strings.HasPrefix(slash, "/") || (len(slash) > 1 && slash[1] == ':') {
		return restoredDatabaseEntry, true, nil
	}
	for _, part := range strings.Split(slash, "/") {
		if part == ".." {
			return "", false, errors.New("database path contains parent traversal")
		}
	}
	clean, err := fsguard.ArchivePath(slash, true)
	if err != nil {
		return "", false, err
	}
	first, _, nested := strings.Cut(clean, "/")
	if !nested || strings.HasPrefix(first, ".") {
		return "", false, errors.New("database must be inside a runtime data directory")
	}
	switch strings.ToLower(first) {
	case "config", "plugins", "logs", "cache", "backups", "templates", "web", ".deps":
		return "", false, errors.New("database path targets a protected directory")
	}
	return clean, false, nil
}

func checkRestoreTarget(root *os.Root, name string) error {
	parts := strings.Split(filepath.ToSlash(name), "/")
	for index := range parts {
		info, err := root.Lstat(filepath.FromSlash(strings.Join(parts[:index+1], "/")))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("restore destination contains a symbolic link")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return errors.New("restore destination parent is not a directory")
		}
		if index == len(parts)-1 && !info.Mode().IsRegular() {
			return errors.New("restore destination is not a regular file")
		}
	}
	return nil
}

func createRestoreParents(root *os.Root, directory string, created *[]string) error {
	if directory == "." {
		return nil
	}
	parts := strings.Split(filepath.ToSlash(directory), "/")
	for index := range parts {
		name := filepath.FromSlash(strings.Join(parts[:index+1], "/"))
		if err := root.Mkdir(name, 0o755); err == nil {
			*created = append(*created, name)
		} else if !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	return nil
}

func rollbackRestore(root *os.Root, writes []restoreWrite, created []string, deps restoreDeps) error {
	var rollbackErr error
	for index := len(writes) - 1; index >= 0; index-- {
		write := writes[index]
		if write.installed {
			if err := deps.remove(root, write.target); err != nil {
				rollbackErr = errors.Join(rollbackErr, err)
				continue
			}
		}
		if write.backedUp {
			rollbackErr = errors.Join(rollbackErr, deps.rename(root, write.previous, write.target))
		}
	}
	for index := len(created) - 1; index >= 0; index-- {
		if err := root.Remove(created[index]); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	return rollbackErr
}

func copyRestoreEntry(ctx context.Context, entry *zip.File, output *os.File) (err error) {
	reader, err := entry.Open()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, reader.Close()) }()
	if err := fsguard.CopyExact(ctx, output, reader, int64(entry.UncompressedSize64)); err != nil {
		return err
	}
	return output.Sync()
}

func inspectRestoreArchive(files []*zip.File) (BackupManifest, map[string]*zip.File, error) {
	var manifest BackupManifest
	if len(files) > maxRestoreEntries {
		return manifest, nil, fmt.Errorf("%w: entry count", errRestoreResourceLimit)
	}
	entries := make(map[string]*zip.File, len(files))
	caseNames := make(map[string]bool, len(files))
	spelling := make(map[string]string, len(files))
	var total uint64
	for _, entry := range files {
		name, err := fsguard.ArchivePath(entry.Name, true)
		if err != nil || name != strings.TrimSuffix(entry.Name, "/") {
			return manifest, nil, errors.New("restore archive contains an unsafe path")
		}
		if caseNames[strings.ToLower(name)] {
			return manifest, nil, errors.New("restore archive contains duplicate or case-colliding paths")
		}
		caseNames[strings.ToLower(name)], entries[name] = true, entry
		for prefix := name; prefix != "."; prefix = path.Dir(prefix) {
			key := strings.ToLower(prefix)
			if previous, exists := spelling[key]; exists && previous != prefix {
				return manifest, nil, errors.New("restore archive contains case-colliding directories")
			}
			spelling[key] = prefix
		}
		if !entry.Mode().IsRegular() && !entry.FileInfo().IsDir() {
			return manifest, nil, errors.New("restore archive contains a symbolic link or special file")
		}
		if entry.UncompressedSize64 > maxRestoreFileBytes {
			return manifest, nil, fmt.Errorf("%w: single file", errRestoreResourceLimit)
		}
		total += entry.UncompressedSize64
		if total > maxRestoreBytes {
			return manifest, nil, fmt.Errorf("%w: expanded size", errRestoreResourceLimit)
		}
		if entry.UncompressedSize64 > 0 && (entry.CompressedSize64 == 0 || (entry.UncompressedSize64+maxRestoreRatio-1)/maxRestoreRatio > entry.CompressedSize64) {
			return manifest, nil, fmt.Errorf("%w: compression ratio", errRestoreResourceLimit)
		}
	}
	manifestEntry := entries["backup-manifest.json"]
	if manifestEntry == nil || !manifestEntry.Mode().IsRegular() || manifestEntry.UncompressedSize64 > 1<<20 {
		return manifest, nil, errors.New("restore archive has no valid manifest")
	}
	reader, err := manifestEntry.Open()
	if err != nil {
		return manifest, nil, err
	}
	decoder := json.NewDecoder(io.LimitReader(reader, (1<<20)+1))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&manifest)
	if err == nil {
		var trailing any
		if decoder.Decode(&trailing) != io.EOF {
			err = errors.New("restore manifest contains trailing data")
		}
	}
	err = errors.Join(err, reader.Close())
	if err != nil {
		return manifest, nil, err
	}
	if err := ValidateBackupManifest(manifest); err != nil {
		return manifest, nil, err
	}
	labels := map[string]bool{}
	for _, directory := range manifest.Directories {
		if labels[directory.Label] {
			return manifest, nil, errors.New("restore manifest repeats a root label")
		}
		labels[directory.Label] = true
	}
	if !labels["config"] || entries["config/user.yaml"] == nil || !entries["config/user.yaml"].Mode().IsRegular() {
		return manifest, nil, errors.New("restore archive is missing the effective configuration")
	}
	if labels["database"] != (manifest.DBSchemaVersion != "absent") {
		return manifest, nil, errors.New("restore database presence differs from manifest")
	}
	if labels["database"] && (entries[restoredDatabaseEntry] == nil || !entries[restoredDatabaseEntry].Mode().IsRegular()) {
		return manifest, nil, errors.New("restore archive is missing its database snapshot")
	}
	for name, entry := range entries {
		if name == "backup-manifest.json" {
			continue
		}
		allowed := name == "config/user.yaml" || (entry.FileInfo().IsDir() && name == "config")
		if name == restoredDatabaseEntry {
			allowed = labels["database"]
		} else if name == "data" || strings.HasPrefix(name, "data/") {
			allowed = labels["data"] || (entry.FileInfo().IsDir() && name == "data" && labels["database"])
		}
		if name == "plugins" && entry.FileInfo().IsDir() || name == "plugins/installed" || strings.HasPrefix(name, "plugins/installed/") {
			allowed = labels["plugins"]
		}
		if !allowed {
			return manifest, nil, errors.New("restore entry is outside the declared archive roots")
		}
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if existing := entries[parent]; existing != nil && !existing.FileInfo().IsDir() {
				return manifest, nil, errors.New("restore archive has a file-directory collision")
			}
		}
	}
	return manifest, entries, nil
}

package backup

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/operations/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

const databaseArchivePath = "data/rayleabot.db"

type SnapshotFunc func(context.Context, string) (string, error)

type ProgressFunc func(percent int, summary string)

type Options struct {
	RepoRoot       string
	ConfigPath     string
	DatabasePath   string
	Consistency    string
	CreateSnapshot SnapshotFunc
	Now            func() time.Time
	Progress       ProgressFunc
}

type Result struct {
	ArchivePath string
	Manifest    recovery.BackupManifest
}

// backupInputs holds the validated, absolute form of Options.
type backupInputs struct {
	repoRoot     string
	configPath   string
	databasePath string
	consistency  string
	now          func() time.Time
}

func normalizeOptions(options Options) (backupInputs, error) {
	if strings.TrimSpace(options.RepoRoot) == "" {
		return backupInputs{}, errors.New("backup runtime root is required")
	}
	repoRoot, err := filepath.Abs(strings.TrimSpace(options.RepoRoot))
	if err != nil {
		return backupInputs{}, fmt.Errorf("resolve backup runtime root: %w", err)
	}
	if strings.TrimSpace(options.ConfigPath) == "" {
		return backupInputs{}, errors.New("backup config path is required")
	}
	configPath, err := filepath.Abs(strings.TrimSpace(options.ConfigPath))
	if err != nil {
		return backupInputs{}, fmt.Errorf("resolve backup config path: %w", err)
	}
	databasePath := ""
	if strings.TrimSpace(options.DatabasePath) != "" {
		databasePath, err = filepath.Abs(options.DatabasePath)
		if err != nil {
			return backupInputs{}, fmt.Errorf("resolve backup database path: %w", err)
		}
	}
	consistency := strings.TrimSpace(options.Consistency)
	if consistency != "offline" && consistency != "online" {
		return backupInputs{}, fmt.Errorf("unsupported backup consistency %q", consistency)
	}
	if options.CreateSnapshot == nil {
		return backupInputs{}, errors.New("database snapshot function is required")
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return backupInputs{repoRoot: repoRoot, configPath: configPath, databasePath: databasePath, consistency: consistency, now: now}, nil
}

func Create(ctx context.Context, options Options) (Result, error) {
	inputs, err := normalizeOptions(options)
	if err != nil {
		return Result{}, err
	}
	repoRoot, configPath, databasePath, consistency := inputs.repoRoot, inputs.configPath, inputs.databasePath, inputs.consistency

	backupDir := filepath.Join(repoRoot, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create backup directory: %w", err)
	}
	archivePath, err := allocateArchivePath(backupDir, inputs.now().UTC())
	if err != nil {
		return Result{}, err
	}
	tempFile, err := os.CreateTemp(backupDir, ".rayleabot-backup-*.partial")
	if err != nil {
		return Result{}, fmt.Errorf("create temporary backup archive: %w", err)
	}
	tempPath := tempFile.Name()
	keepTemp := true
	defer func() {
		_ = tempFile.Close()
		if keepTemp {
			_ = os.Remove(tempPath)
		}
	}()

	writer := zip.NewWriter(tempFile)
	closed := false
	closeArchive := func() error {
		if closed {
			return nil
		}
		closed = true
		if err := writer.Close(); err != nil {
			return err
		}
		return tempFile.Close()
	}
	defer func() {
		if !closed {
			_ = writer.Close()
		}
	}()

	progress(options.Progress, 10, "写入配置")
	configVersion, err := addConfigSnapshot(writer, configPath)
	if err != nil {
		return Result{}, fmt.Errorf("archive config: %w", err)
	}
	databaseVersion := "absent"
	directories := []recovery.BackupManifestDirectory{
		recovery.Directory("config/user.yaml", "config"),
	}

	if databasePath != "" {
		version, added, err := addDatabaseSnapshot(ctx, writer, databasePath, options)
		if err != nil {
			return Result{}, err
		}
		if added {
			databaseVersion = version
			directories = append(directories, recovery.Directory(databaseArchivePath, "database"))
		}
	}

	progress(options.Progress, 55, "写入运行数据")
	snapshotRoot := ""
	if databasePath != "" {
		snapshotRoot = storage.SnapshotDirForDatabase(databasePath)
	}
	skipDatabaseFiles := func(sourcePath, archivePath string, entry fs.DirEntry) bool {
		if samePath(sourcePath, databasePath) || samePath(sourcePath, databasePath+"-wal") || samePath(sourcePath, databasePath+"-shm") || samePath(sourcePath, databasePath+".lock") {
			return true
		}
		if snapshotRoot != "" && samePath(sourcePath, snapshotRoot) {
			return true
		}
		return archivePath == databaseArchivePath && !entry.IsDir()
	}
	added, err := addOptionalDirectory(ctx, writer, filepath.Join(repoRoot, "data"), "data", "runtime data", skipDatabaseFiles)
	if err != nil {
		return Result{}, err
	}
	if added {
		directories = append(directories, recovery.Directory("data", "data"))
	}

	progress(options.Progress, 75, "写入插件产物")
	added, err = addOptionalDirectory(ctx, writer, filepath.Join(repoRoot, "plugins", "installed"), "plugins/installed", "installed plugins", nil)
	if err != nil {
		return Result{}, err
	}
	if added {
		directories = append(directories, recovery.Directory("plugins/installed", "plugins"))
	}

	progress(options.Progress, 90, "写入备份清单")
	manifest := recovery.BuildBackupManifest(repoRoot, consistency)
	manifest.ConfigSchemaVersion = configVersion
	manifest.DBSchemaVersion = databaseVersion
	manifest.Directories = directories
	if err := recovery.ValidateBackupManifest(manifest); err != nil {
		return Result{}, fmt.Errorf("validate backup manifest: %w", err)
	}
	if err := addJSON(writer, "backup-manifest.json", manifest); err != nil {
		return Result{}, fmt.Errorf("archive backup manifest: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := closeArchive(); err != nil {
		return Result{}, fmt.Errorf("finalize backup archive: %w", err)
	}
	if err := os.Rename(tempPath, archivePath); err != nil {
		return Result{}, fmt.Errorf("publish backup archive: %w", err)
	}
	keepTemp = false
	progress(options.Progress, 100, "备份完成")
	return Result{ArchivePath: archivePath, Manifest: manifest}, nil
}

func addConfigSnapshot(writer *zip.Writer, path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("backup config is not a regular file: %s", path)
	}
	document, err := config.LoadDocument(path, "")
	if err != nil {
		return "", err
	}
	payload, err := config.MarshalDocument(document)
	if err != nil {
		return "", err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return "", err
	}
	header.Name, header.Method = "config/user.yaml", zip.Deflate
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return "", err
	}
	if _, err := entry.Write(payload); err != nil {
		return "", err
	}
	version, _ := document["schema_version"].(string)
	if version != config.CurrentSchemaVersion() {
		return "", fmt.Errorf("archived configuration must declare the current schema version")
	}
	return version, nil
}

func allocateArchivePath(directory string, now time.Time) (string, error) {
	base := "backup-" + now.Format("20060102-150405.000000000")
	for index := 0; index < 100; index++ {
		name := base
		if index > 0 {
			name = fmt.Sprintf("%s-%02d", base, index)
		}
		candidate := filepath.Join(directory, name+".zip")
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("stat backup archive path: %w", err)
		}
	}
	return "", errors.New("allocate backup archive path: too many collisions")
}

func addJSON(writer *zip.Writer, archivePath string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	entry, err := writer.Create(filepath.ToSlash(archivePath))
	if err != nil {
		return err
	}
	_, err = entry.Write(payload)
	return err
}

func addFile(ctx context.Context, writer *zip.Writer, sourcePath, archivePath string) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("backup source is not a regular file: %s", sourcePath)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func(release func() error) { _ = release() }(source.Close)
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(archivePath)
	header.Method = zip.Deflate
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = copyContext(ctx, entry, source)
	return err
}

// addDatabaseSnapshot archives a consistent snapshot of the database file and
// returns its schema version; a missing database file adds nothing.
func addDatabaseSnapshot(ctx context.Context, writer *zip.Writer, databasePath string, options Options) (string, bool, error) {
	info, err := os.Stat(databasePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("stat database: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", false, nil
	}
	progress(options.Progress, 30, "创建数据库快照")
	snapshotPath, err := options.CreateSnapshot(ctx, databasePath)
	if err != nil {
		return "", false, fmt.Errorf("create database snapshot: %w", err)
	}
	version, err := storage.ReadSchemaVersion(ctx, snapshotPath)
	if err != nil {
		return "", false, fmt.Errorf("read database snapshot version: %w", err)
	}
	if err := addFile(ctx, writer, snapshotPath, databaseArchivePath); err != nil {
		return "", false, fmt.Errorf("archive database snapshot: %w", err)
	}
	return version, true, nil
}

type skipFunc func(sourcePath, archivePath string, entry fs.DirEntry) bool

// addOptionalDirectory archives sourceRoot when it exists as a directory and
// reports whether anything was added; a missing directory is not an error.
func addOptionalDirectory(ctx context.Context, writer *zip.Writer, sourceRoot, archivePrefix, label string, skip skipFunc) (bool, error) {
	info, err := os.Stat(sourceRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", label, err)
	}
	if !info.IsDir() {
		return false, nil
	}
	if err := addDirectory(ctx, writer, sourceRoot, archivePrefix, skip); err != nil {
		return false, fmt.Errorf("archive %s: %w", label, err)
	}
	return true, nil
}

func addDirectory(ctx context.Context, writer *zip.Writer, sourceRoot, archivePrefix string, skip skipFunc) error {
	return filepath.WalkDir(sourceRoot, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relativePath, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		archivePath := filepath.ToSlash(filepath.Join(archivePrefix, relativePath))
		if skip != nil && skip(sourcePath, archivePath, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			_, err := writer.Create(strings.TrimSuffix(archivePath, "/") + "/")
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backup source contains a non-regular file: %s", sourcePath)
		}
		if err := addFile(ctx, writer, sourcePath, archivePath); err != nil {
			return err
		}
		return nil
	})
}

func copyContext(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	buffer := make([]byte, 64*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			written, writeErr := destination.Write(buffer[:read])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
			if written != read {
				return total, io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
	}
}

func samePath(left, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	leftAbsolute, leftErr := filepath.Abs(left)
	rightAbsolute, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return strings.EqualFold(filepath.Clean(leftAbsolute), filepath.Clean(rightAbsolute))
}

func progress(callback ProgressFunc, percent int, summary string) {
	if callback != nil {
		callback(percent, summary)
	}
}

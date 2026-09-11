package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/fsguard"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func (w *restoreWorkspace) prepareConfig(manifest BackupManifest) (string, bool, error) {
	root, repoRoot, work := w.root, w.repoRoot, w.work
	stagedConfig := filepath.Join(work, "incoming", "config", "user.yaml")
	document, err := config.LoadDocument(filepath.Join(repoRoot, stagedConfig), "")
	if err != nil {
		return "", false, &RestoreError{Stage: "config", cause: err}
	}
	if document["schema_version"] != manifest.ConfigSchemaVersion {
		return "", false, errors.New("archived configuration version does not match manifest")
	}
	databaseDocument, ok := document["database"].(map[string]any)
	if !ok {
		return "", false, errors.New("archived configuration has no database definition")
	}
	sourceDatabase, _ := databaseDocument["path"].(string)
	databaseRelative, relocated, err := portableDatabasePath(sourceDatabase)
	if err != nil {
		return "", false, &RestoreError{Stage: "database path", cause: err}
	}
	databaseDocument["path"] = databaseRelative
	configPayload, err := config.MarshalDocument(document)
	if err != nil {
		return "", false, err
	}
	if err := root.WriteFile(stagedConfig, configPayload, 0o600); err != nil {
		return "", false, err
	}
	return databaseRelative, relocated, nil
}

func (w *restoreWorkspace) verifyDatabase(ctx context.Context, manifest BackupManifest) error {
	repoRoot, work := w.repoRoot, w.work
	hasDatabase := manifest.DBSchemaVersion != "absent"
	if hasDatabase {
		stagedDatabase := filepath.Join(repoRoot, work, "incoming", filepath.FromSlash(restoredDatabaseEntry))
		if err := storage.QuickCheckPath(ctx, stagedDatabase); err != nil {
			return &RestoreError{Stage: "database", cause: err}
		}
		version, err := storage.ReadSchemaVersion(ctx, stagedDatabase)
		if err != nil || version != manifest.DBSchemaVersion {
			return &RestoreError{Stage: "database metadata", cause: errors.Join(err, errors.New("snapshot version differs from manifest"))}
		}
	}
	return nil
}

func (w *restoreWorkspace) saveSummary(summary CompatibilitySummary) error {
	payload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	return w.root.WriteFile(filepath.Join(w.work, "summary.json"), append(payload, '\n'), 0o600)
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

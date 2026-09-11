package recovery

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/fsguard"
)

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
	index := restoreArchiveIndex{entries: make(map[string]*zip.File, len(files)), caseNames: make(map[string]bool, len(files)), spelling: make(map[string]string)}
	for _, entry := range files {
		if err := index.add(entry); err != nil {
			return manifest, nil, err
		}
	}
	manifest, err := readRestoreManifest(index.entries["backup-manifest.json"])
	if err != nil {
		return manifest, nil, err
	}
	labels, err := restoreRootLabels(manifest, index.entries)
	if err != nil {
		return manifest, nil, err
	}
	if err := validateRestoreRoots(index.entries, labels); err != nil {
		return manifest, nil, err
	}
	return manifest, index.entries, nil
}

type restoreArchiveIndex struct {
	entries   map[string]*zip.File
	caseNames map[string]bool
	spelling  map[string]string
	total     uint64
}

func (index *restoreArchiveIndex) add(entry *zip.File) error {
	name, err := fsguard.ArchivePath(entry.Name, true)
	if err != nil || name != strings.TrimSuffix(entry.Name, "/") {
		return errors.New("restore archive contains an unsafe path")
	}
	if index.caseNames[strings.ToLower(name)] {
		return errors.New("restore archive contains duplicate or case-colliding paths")
	}
	if err := index.addSpelling(name); err != nil {
		return err
	}
	if !entry.Mode().IsRegular() && !entry.FileInfo().IsDir() {
		return errors.New("restore archive contains a symbolic link or special file")
	}
	if entry.UncompressedSize64 > maxRestoreFileBytes {
		return fmt.Errorf("%w: single file", errRestoreResourceLimit)
	}
	index.total += entry.UncompressedSize64
	if index.total > maxRestoreBytes {
		return fmt.Errorf("%w: expanded size", errRestoreResourceLimit)
	}
	if entry.UncompressedSize64 > 0 && (entry.CompressedSize64 == 0 || (entry.UncompressedSize64+maxRestoreRatio-1)/maxRestoreRatio > entry.CompressedSize64) {
		return fmt.Errorf("%w: compression ratio", errRestoreResourceLimit)
	}
	index.caseNames[strings.ToLower(name)] = true
	index.entries[name] = entry
	return nil
}

func (index *restoreArchiveIndex) addSpelling(name string) error {
	for prefix := name; prefix != "."; prefix = path.Dir(prefix) {
		key := strings.ToLower(prefix)
		if previous, exists := index.spelling[key]; exists && previous != prefix {
			return errors.New("restore archive contains case-colliding directories")
		}
		index.spelling[key] = prefix
	}
	return nil
}

func readRestoreManifest(entry *zip.File) (BackupManifest, error) {
	var manifest BackupManifest
	if entry == nil || !entry.Mode().IsRegular() || entry.UncompressedSize64 > 1<<20 {
		return manifest, errors.New("restore archive has no valid manifest")
	}
	reader, err := entry.Open()
	if err != nil {
		return manifest, err
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
	if err := errors.Join(err, reader.Close()); err != nil {
		return manifest, err
	}
	return manifest, ValidateBackupManifest(manifest)
}

func restoreRootLabels(manifest BackupManifest, entries map[string]*zip.File) (map[string]bool, error) {
	labels := map[string]bool{}
	for _, directory := range manifest.Directories {
		if labels[directory.Label] {
			return nil, errors.New("restore manifest repeats a root label")
		}
		labels[directory.Label] = true
	}
	if !labels["config"] || !regularRestoreEntry(entries, "config/user.yaml") {
		return nil, errors.New("restore archive is missing the effective configuration")
	}
	if labels["database"] != (manifest.DBSchemaVersion != "absent") {
		return nil, errors.New("restore database presence differs from manifest")
	}
	if labels["database"] && !regularRestoreEntry(entries, restoredDatabaseEntry) {
		return nil, errors.New("restore archive is missing its database snapshot")
	}
	return labels, nil
}

func regularRestoreEntry(entries map[string]*zip.File, name string) bool {
	return entries[name] != nil && entries[name].Mode().IsRegular()
}

func validateRestoreRoots(entries map[string]*zip.File, labels map[string]bool) error {
	for name, entry := range entries {
		if name == "backup-manifest.json" {
			continue
		}
		if !restoreEntryDeclared(name, entry.FileInfo().IsDir(), labels) {
			return errors.New("restore entry is outside the declared archive roots")
		}
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if existing := entries[parent]; existing != nil && !existing.FileInfo().IsDir() {
				return errors.New("restore archive has a file-directory collision")
			}
		}
	}
	return nil
}

func restoreEntryDeclared(name string, directory bool, labels map[string]bool) bool {
	switch {
	case name == restoredDatabaseEntry:
		return labels["database"]
	case name == "data" || strings.HasPrefix(name, "data/"):
		return labels["data"] || (directory && name == "data" && labels["database"])
	case name == "plugins" && directory || name == "plugins/installed" || strings.HasPrefix(name, "plugins/installed/"):
		return labels["plugins"]
	default:
		return name == "config/user.yaml" || (directory && name == "config")
	}
}

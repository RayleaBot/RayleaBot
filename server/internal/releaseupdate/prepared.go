package releaseupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/filelock"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/fsguard"
)

type preparedUpdate struct {
	Result   CheckResult `json:"result"`
	Staging  string      `json:"staging"`
	Applying bool        `json:"applying"`
}

func readPrepared(root string) (preparedUpdate, error) {
	var prepared preparedUpdate
	payload, err := os.ReadFile(filepath.Join(root, "cache", "update", "prepared.json"))
	if err != nil {
		return prepared, err
	}
	if err = json.Unmarshal(payload, &prepared); err != nil {
		return prepared, err
	}
	if !fileNamePattern.MatchString(prepared.Staging) || !strings.HasPrefix(prepared.Staging, "staging-") {
		return prepared, errors.New("invalid prepared staging directory")
	}
	if !fileNamePattern.MatchString(prepared.Result.Artifact.FileName) {
		return prepared, errors.New("invalid prepared archive name")
	}
	if _, err = parseSemanticVersion(prepared.Result.AvailableVersion); err != nil {
		return prepared, err
	}
	if _, err = parseSemanticVersion(prepared.Result.CurrentVersion); err != nil {
		return prepared, err
	}
	if !fileNamePattern.MatchString(prepared.Result.Artifact.ArtifactID) {
		return prepared, errors.New("invalid prepared artifact")
	}
	return prepared, nil
}

func writePrepared(root string, prepared preparedUpdate) error {
	payload, err := json.Marshal(prepared)
	if err != nil {
		return err
	}
	return fsguard.WriteFileAtomic(filepath.Join(root, "cache", "update", "prepared.json"), payload, 0600)
}

func (c *Checker) prepareArchive(ctx context.Context, root, archive string, result CheckResult) (CheckResult, error) {
	directory, err := os.MkdirTemp(filepath.Join(root, "cache", "update"), "staging-")
	if err != nil {
		return result, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(directory)
		}
	}()
	c.report("verify", "", 0, 0)
	if _, err := stageRelease(ctx, archive, directory, result); err != nil {
		return result, err
	}
	prepared, err := c.recordPrepared(root, directory, result)
	keep = err == nil
	return prepared, err
}

func (c *Checker) recordPrepared(root, directory string, result CheckResult) (CheckResult, error) {
	previous, _ := readPrepared(root)
	prepared := preparedUpdate{Result: result, Staging: filepath.Base(directory)}
	if err := writePrepared(root, prepared); err != nil {
		return result, err
	}
	if previous.Staging != "" && previous.Staging != prepared.Staging {
		_ = os.RemoveAll(filepath.Join(root, "cache", "update", previous.Staging))
		if previous.Result.Artifact.FileName != result.Artifact.FileName {
			_ = os.Remove(filepath.Join(root, "cache", "downloads", "update", previous.Result.Artifact.FileName))
		}
	}
	c.report("ready", "", result.Artifact.ArchiveSizeBytes, result.Artifact.ArchiveSizeBytes)
	result.PreparedID = prepared.Staging
	return result, nil
}

func discardPrepared(root string) error {
	previous, err := readPrepared(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if previous.Applying {
		return errors.New("an interrupted local update must be completed with update apply first")
	}
	if err := os.Remove(filepath.Join(root, "cache", "update", "prepared.json")); err != nil {
		return err
	}
	return errors.Join(os.RemoveAll(filepath.Join(root, "cache", "update", previous.Staging)), os.RemoveAll(filepath.Join(root, "cache", "downloads", "update", previous.Result.Artifact.FileName)))
}

// Import prepares a local release archive using the same validation and apply
// path as an online download, without making any network request.
func (c *Checker) Import(ctx context.Context, root, source string) (CheckResult, error) {
	lock, err := filelock.Acquire(filepath.Join(root, "cache", "update", "operation.lock"))
	if err != nil {
		return CheckResult{}, err
	}
	defer func() { _ = lock.Close() }()
	format := "zip"
	if strings.HasSuffix(source, ".tar.gz") {
		format = "tar.gz"
	} else if !strings.HasSuffix(source, ".zip") {
		return CheckResult{}, errors.New("expected .zip or .tar.gz release archive")
	}
	input, err := os.Open(source)
	if err != nil {
		return CheckResult{}, err
	}
	defer func() { _ = input.Close() }()
	info, err := input.Stat()
	if err != nil {
		return CheckResult{}, err
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 2<<30 {
		return CheckResult{}, errors.New("invalid release archive size")
	}
	directory := filepath.Join(root, "cache", "downloads", "update")
	if err := os.MkdirAll(directory, 0755); err != nil {
		return CheckResult{}, err
	}
	output, err := os.CreateTemp(directory, "import-*."+format)
	if err != nil {
		return CheckResult{}, err
	}
	temporary := output.Name()
	defer func() { _ = os.Remove(temporary) }()
	// Validate the cached copy so preparation and later retries use the same
	// bytes even if the user replaces the source archive during import.
	written, copyErr := fsguard.CopyAtMost(ctx, output, input, info.Size())
	if err := errors.Join(copyErr, input.Close(), output.Sync(), output.Close()); err != nil {
		return CheckResult{}, err
	}
	if written != info.Size() {
		return CheckResult{}, errors.New("local archive changed while copying")
	}
	stage, err := os.MkdirTemp(filepath.Join(root, "cache", "update"), "staging-")
	if err != nil {
		return CheckResult{}, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(stage)
		}
	}()
	c.report("verify", "", 0, 0)
	if err := deps.ExtractWithProgress(ctx, temporary, format, stage, nil); err != nil {
		return CheckResult{}, err
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return CheckResult{}, err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return CheckResult{}, errors.New("release must have one root directory")
	}
	payload, err := os.ReadFile(filepath.Join(stage, entries[0].Name(), "build_info.json"))
	if err != nil {
		return CheckResult{}, err
	}
	target, err := DecodeBuildInfo(payload)
	if err != nil {
		return CheckResult{}, err
	}
	payload, err = os.ReadFile(filepath.Join(root, "build_info.json"))
	if err != nil {
		return CheckResult{}, err
	}
	current, err := DecodeBuildInfo(payload)
	if err != nil {
		return CheckResult{}, err
	}
	cmp, err := compareSemanticVersions(target.Version, current.Version)
	if err != nil || cmp <= 0 || target.ArtifactID != current.ArtifactID {
		return CheckResult{}, errors.New("local archive must be a newer version of the installed artifact")
	}
	name := "RayleaBot-v" + target.Version + "-" + target.ArtifactID + "." + format
	if !fileNamePattern.MatchString(name) {
		return CheckResult{}, errors.New("invalid release file name")
	}
	result := CheckResult{Status: "update_available", CurrentVersion: current.Version, AvailableVersion: target.Version, UpdateMode: "guided", ReleasePageURL: ReleaseRepositoryURL + "/releases/tag/v" + target.Version, Artifact: Artifact{ArtifactID: target.ArtifactID, FileName: name, ArchiveSizeBytes: info.Size(), UpdateMode: "guided"}}
	if entries[0].Name() != strings.TrimSuffix(name, "."+format) {
		return result, errors.New("release root does not match build information")
	}
	if err := validateStagedRelease(filepath.Join(stage, entries[0].Name()), result); err != nil {
		return result, err
	}
	archive := filepath.Join(directory, name)
	if err := os.Rename(temporary, archive); err != nil {
		return result, fmt.Errorf("cache local archive: %w", err)
	}
	prepared, err := c.recordPrepared(root, stage, result)
	keep = err == nil
	return prepared, err
}

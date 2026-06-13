package recovery

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

func SummaryPath(repoRoot string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(RecoverySummaryPath))
}

func LoadSummary(repoRoot string) (*CompatibilitySummary, error) {
	path := SummaryPath(repoRoot)
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var summary CompatibilitySummary
	if err := json.Unmarshal(payload, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

func SaveSummary(repoRoot string, summary CompatibilitySummary) error {
	path := SummaryPath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func RemoveSummary(repoRoot string) error {
	err := os.Remove(SummaryPath(repoRoot))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func HasSummary(repoRoot string) bool {
	_, err := os.Stat(SummaryPath(repoRoot))
	return err == nil
}

func LoadSummaryFromFile(path string) (*CompatibilitySummary, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var summary CompatibilitySummary
	if err := json.Unmarshal(payload, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

func SaveSummaryToFile(path string, summary CompatibilitySummary) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func AvailableRecoveryLogFiles(logDir string) []fs.DirEntry {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil
	}
	return entries
}

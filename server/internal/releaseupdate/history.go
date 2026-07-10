package releaseupdate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type observationState struct {
	HighestVersion string            `json:"highest_version"`
	Digests        map[string]string `json:"manifest_digests"`
	UpdatedAt      string            `json:"updated_at"`
}

func ObserveManifest(historyPath string, current BuildInfo, verified VerifiedManifest, now time.Time) error {
	return withHistoryLock(historyPath, func() error {
		state, err := loadObservationState(historyPath)
		if err != nil {
			return errorWithCode(CodeReplayRejected, "load update observation history", err)
		}
		if state.Digests == nil {
			state.Digests = make(map[string]string)
		}

		if current.ReleaseManifestSHA256 != "" {
			if existing := state.Digests[current.Version]; existing != "" && existing != current.ReleaseManifestSHA256 {
				return errorWithCode(CodeReplayRejected, "validate installed release digest", errors.New("installed release digest conflicts with observation history"))
			}
			state.Digests[current.Version] = current.ReleaseManifestSHA256
		}
		if state.HighestVersion == "" {
			state.HighestVersion = current.Version
		} else if comparison, compareErr := compareSemanticVersions(state.HighestVersion, current.Version); compareErr != nil {
			return errorWithCode(CodeReplayRejected, "compare installed version", compareErr)
		} else if comparison < 0 {
			state.HighestVersion = current.Version
		}

		if comparison, compareErr := compareSemanticVersions(verified.Manifest.Version, current.Version); compareErr != nil {
			return errorWithCode(CodeManifestInvalid, "compare manifest version", compareErr)
		} else if comparison < 0 {
			return errorWithCode(CodeReplayRejected, "reject version downgrade", fmt.Errorf("manifest version %s is older than installed version %s", verified.Manifest.Version, current.Version))
		}

		if existing := state.Digests[verified.Manifest.Version]; existing != "" && existing != verified.Digest {
			return errorWithCode(CodeReplayRejected, "reject replaced manifest", fmt.Errorf("version %s was already observed with a different digest", verified.Manifest.Version))
		}
		if comparison, compareErr := compareSemanticVersions(verified.Manifest.Version, state.HighestVersion); compareErr != nil {
			return errorWithCode(CodeReplayRejected, "compare highest observed version", compareErr)
		} else if comparison < 0 {
			return errorWithCode(CodeReplayRejected, "reject replayed manifest", fmt.Errorf("manifest version %s is older than highest observed version %s", verified.Manifest.Version, state.HighestVersion))
		} else if comparison > 0 {
			state.HighestVersion = verified.Manifest.Version
		}

		state.Digests[verified.Manifest.Version] = verified.Digest
		state.UpdatedAt = now.UTC().Format(time.RFC3339)
		if err := saveJSONAtomically(historyPath, state, 0o600); err != nil {
			return errorWithCode(CodeReplayRejected, "persist update observation history", err)
		}
		return nil
	})
}

func loadObservationState(historyPath string) (observationState, error) {
	data, err := os.ReadFile(historyPath)
	if errors.Is(err, os.ErrNotExist) {
		return observationState{Digests: make(map[string]string)}, nil
	}
	if err != nil {
		return observationState{}, err
	}
	var state observationState
	if err := decodeStrictJSON(data, &state); err != nil {
		return observationState{}, err
	}
	if state.HighestVersion != "" {
		if _, err := parseSemanticVersion(state.HighestVersion); err != nil {
			return observationState{}, fmt.Errorf("invalid highest observed version: %w", err)
		}
	}
	for version, digest := range state.Digests {
		if _, err := parseSemanticVersion(version); err != nil || !sha256Pattern.MatchString(digest) {
			return observationState{}, fmt.Errorf("invalid observed release entry")
		}
	}
	return state, nil
}

func withHistoryLock(historyPath string, operation func() error) error {
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o700); err != nil {
		return err
	}
	lockPath := historyPath + ".lock"
	deadline := time.Now().Add(5 * time.Second)
	for {
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(lock, "%d\n", os.Getpid())
			_ = lock.Close()
			defer os.Remove(lockPath)
			return operation()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > time.Minute {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for update history lock")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func saveJSONAtomically(path string, value any, mode os.FileMode) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(payload); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

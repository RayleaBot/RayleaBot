package releaseupdate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrCheckInProgress = errors.New("an update check is already in progress")

type CheckProvider interface {
	Check(context.Context, string) (CheckResult, error)
}

type StatusSnapshot struct {
	State            string     `json:"state"`
	CurrentVersion   string     `json:"current_version"`
	AvailableVersion string     `json:"available_version,omitempty"`
	CheckedAt        *time.Time `json:"checked_at"`
	UpdateMode       string     `json:"update_mode"`
	ReleaseNotesRef  string     `json:"release_notes_ref,omitempty"`
	ErrorCode        string     `json:"-"`
}

type Service struct {
	mu          sync.RWMutex
	installRoot string
	checker     CheckProvider
	now         func() time.Time
	checking    bool
	snapshot    StatusSnapshot
}

func NewService(installRoot string, checker CheckProvider, currentVersion string) *Service {
	state := "idle"
	mode := "guided"
	errorCode := ""
	if checker == nil {
		state = "disabled"
		mode = "unavailable"
		errorCode = CodeManifestInvalid
	}
	if currentVersion == "" {
		currentVersion = "unknown"
	}
	return &Service{
		installRoot: installRoot,
		checker:     checker,
		now:         time.Now,
		snapshot: StatusSnapshot{
			State:          state,
			CurrentVersion: currentVersion,
			UpdateMode:     mode,
			ErrorCode:      errorCode,
		},
	}
}

// InstalledVersion reads validated release metadata for status, backup and
// compatibility checks. It does not substitute a release for a development tree.
func InstalledVersion(installRoot string) string {
	if payload, err := os.ReadFile(filepath.Join(installRoot, "build_info.json")); err == nil {
		if buildInfo, decodeErr := DecodeBuildInfo(payload); decodeErr == nil {
			return buildInfo.Version
		}
	}
	return "unknown"
}

func NewDefaultService(installRoot string) *Service {
	currentVersion := InstalledVersion(installRoot)
	return NewService(installRoot, NewChecker(), currentVersion)
}

func (s *Service) Status() StatusSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneStatusSnapshot(s.snapshot)
}

func (s *Service) Check(ctx context.Context) (StatusSnapshot, error) {
	s.mu.Lock()
	if s.checker == nil {
		snapshot := cloneStatusSnapshot(s.snapshot)
		s.mu.Unlock()
		return snapshot, errorWithCode(CodeManifestInvalid, "check update", errors.New("update checking is unavailable"))
	}
	if s.checking {
		snapshot := cloneStatusSnapshot(s.snapshot)
		s.mu.Unlock()
		return snapshot, ErrCheckInProgress
	}
	s.checking = true
	s.snapshot.State = "checking"
	s.snapshot.ErrorCode = ""
	s.mu.Unlock()

	result, err := s.checker.Check(ctx, s.installRoot)
	checkedAt := s.nowUTC()
	s.mu.Lock()
	s.checking = false
	s.snapshot.CheckedAt = &checkedAt
	if err != nil {
		s.snapshot.State = "failed"
		s.snapshot.UpdateMode = "unavailable"
		s.snapshot.ErrorCode = CodeOf(err)
		if s.snapshot.ErrorCode == "" {
			s.snapshot.ErrorCode = CodeManifestInvalid
		}
		snapshot := cloneStatusSnapshot(s.snapshot)
		s.mu.Unlock()
		return snapshot, err
	}
	s.snapshot.State = result.Status
	s.snapshot.CurrentVersion = result.CurrentVersion
	s.snapshot.AvailableVersion = result.AvailableVersion
	s.snapshot.UpdateMode = result.UpdateMode
	s.snapshot.ReleaseNotesRef = result.ReleasePageURL
	s.snapshot.ErrorCode = ""
	snapshot := cloneStatusSnapshot(s.snapshot)
	s.mu.Unlock()
	return snapshot, nil
}

func (s *Service) nowUTC() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func cloneStatusSnapshot(snapshot StatusSnapshot) StatusSnapshot {
	cloned := snapshot
	if snapshot.CheckedAt != nil {
		checkedAt := *snapshot.CheckedAt
		cloned.CheckedAt = &checkedAt
	}
	return cloned
}

package desktop

import (
	"errors"
	"reflect"
	"testing"
)

type resumeServiceFixture struct {
	snapshot LauncherSnapshot
	starts   int
	err      error
}

func (s *resumeServiceFixture) GetSnapshot() (LauncherSnapshot, error) { return s.snapshot, nil }
func (s *resumeServiceFixture) Start() error                           { s.starts++; return s.err }

func TestResumeFlagAndOneAttempt(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		ownership LauncherProcessOwnership
		lifecycle LauncherProcessLifecycle
		want      int
	}{
		{"absent", []string{WaitForPIDFlag, "123"}, OwnershipNone, Stopped, 0},
		{"explicit", []string{WaitForPIDFlag, "123", ResumeServiceFlag}, OwnershipNone, Stopped, 1},
		{"repeated", []string{ResumeServiceFlag, ResumeServiceFlag}, OwnershipNone, Stopped, 1},
		{"not exact", []string{"--resume-service=false"}, OwnershipNone, Stopped, 0},
		{"external", []string{ResumeServiceFlag}, OwnershipExternal, Stopped, 0},
		{"watcher", []string{ResumeServiceFlag}, OwnershipExternal, Starting, 0},
		{"already managed", []string{ResumeServiceFlag}, OwnershipLauncher, Running, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &resumeServiceFixture{snapshot: defaultSnapshot(), err: errors.New("start failed")}
			s.snapshot.Launcher.ProcessOwnership, s.snapshot.Launcher.ProcessLifecycle = tc.ownership, tc.lifecycle
			resume := NewServiceResume(tc.args)
			for range 2 {
				err := resume.AfterInitialize(s)
				if tc.want == 1 && !errors.Is(err, s.err) {
					t.Fatalf("lost start failure: %v", err)
				}
			}
			if s.starts != tc.want {
				t.Fatalf("starts = %d, want %d", s.starts, tc.want)
			}
		})
	}
}

func TestRelaunchArgumentsPreserveWaitAndOnlyAddExplicitResume(t *testing.T) {
	for _, resume := range []bool{false, true} {
		want := []string{WaitForPIDFlag, "123"}
		if resume {
			want = append(want, ResumeServiceFlag)
		}
		if got := relaunchArguments(123, resume); !reflect.DeepEqual(got, want) {
			t.Fatalf("args = %v, want %v", got, want)
		}
	}
}

func TestResumeUsesNormalStartGateAndFailureSnapshot(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		s := NewService(t.TempDir(), "", 0, &testServiceHost{})
		s.coordinator.initialized = true
		s.coordinator.settings = LauncherSettings{InstallationRoot: t.TempDir()}
		if blocked {
			s.coordinator.startups.block(true)
		}
		err := NewServiceResume([]string{ResumeServiceFlag}).AfterInitialize(s)
		if blocked {
			if !errors.Is(err, errStartupBlocked) {
				t.Fatalf("resume bypassed startup gate: %v", err)
			}
		} else if err != nil || s.coordinator.Snapshot().Launcher.StatusHint == "" {
			t.Fatalf("normal preflight error was not published: %v", err)
		}
	}
}

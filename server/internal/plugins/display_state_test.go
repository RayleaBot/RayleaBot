package plugins

import "testing"

func TestProjectDisplayStateCoversFormalLifecycleEnum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		snapshot Snapshot
		want     string
	}{
		{
			name: "discovered",
			snapshot: Snapshot{
				Valid:             true,
				RegistrationState: stateInstalled,
				DesiredState:      stateDisabled,
				RuntimeState:      stateStopped,
				DisplayState:      displayDiscovered,
			},
			want: displayDiscovered,
		},
		{
			name: "invalid manifest",
			snapshot: Snapshot{
				Valid:             false,
				RegistrationState: stateInstalled,
				DisplayState:      displayInvalid,
			},
			want: displayInvalid,
		},
		{
			name: "unknown display state",
			snapshot: Snapshot{
				Valid:             true,
				RegistrationState: stateInstalled,
				DisplayState:      "paused",
			},
			want: displayInvalid,
		},
		{
			name: "conflict",
			snapshot: Snapshot{
				Valid:             false,
				RegistrationState: stateInstalled,
				DisplayState:      displayConflict,
			},
			want: displayConflict,
		},
		{
			name: "removed",
			snapshot: Snapshot{
				Valid:             true,
				RegistrationState: stateRemoved,
			},
			want: displayRemoved,
		},
		{
			name: "enabled",
			snapshot: Snapshot{
				Valid:             true,
				RegistrationState: stateInstalled,
				DesiredState:      "enabled",
				RuntimeState:      stateStopped,
			},
			want: displayEnabled,
		},
		{
			name: "enabling",
			snapshot: Snapshot{
				Valid:             true,
				RegistrationState: stateInstalled,
				DesiredState:      "enabled",
				RuntimeState:      "starting",
			},
			want: displayEnabling,
		},
		{
			name: "running",
			snapshot: Snapshot{
				Valid:             true,
				RegistrationState: stateInstalled,
				DesiredState:      "enabled",
				RuntimeState:      "running",
			},
			want: displayRunning,
		},
		{
			name: "disabling",
			snapshot: Snapshot{
				Valid:             true,
				RegistrationState: stateInstalled,
				DesiredState:      stateDisabled,
				RuntimeState:      "stopping",
			},
			want: displayDisabling,
		},
		{
			name: "stopping",
			snapshot: Snapshot{
				Valid:             true,
				RegistrationState: stateInstalled,
				DesiredState:      "enabled",
				RuntimeState:      "stopping",
			},
			want: displayStopping,
		},
		{
			name: "crashed",
			snapshot: Snapshot{
				Valid:             true,
				RegistrationState: stateInstalled,
				DesiredState:      "enabled",
				RuntimeState:      "crashed",
			},
			want: displayCrashed,
		},
		{
			name: "backoff",
			snapshot: Snapshot{
				Valid:             true,
				RegistrationState: stateInstalled,
				DesiredState:      "enabled",
				RuntimeState:      "backoff",
			},
			want: displayBackoff,
		},
		{
			name: "dead letter",
			snapshot: Snapshot{
				Valid:             true,
				RegistrationState: stateInstalled,
				DesiredState:      "enabled",
				RuntimeState:      "dead_letter",
			},
			want: displayDeadLetter,
		},
		{
			name: "disabled",
			snapshot: Snapshot{
				Valid:             true,
				RegistrationState: stateInstalled,
				DesiredState:      stateDisabled,
				RuntimeState:      stateStopped,
			},
			want: displayDisabled,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := projectDisplayState(tt.snapshot); got != tt.want {
				t.Fatalf("projectDisplayState() = %q, want %q", got, tt.want)
			}
			if got := cloneSnapshot(tt.snapshot).DisplayState; got != tt.want {
				t.Fatalf("cloneSnapshot().DisplayState = %q, want %q", got, tt.want)
			}
		})
	}
}

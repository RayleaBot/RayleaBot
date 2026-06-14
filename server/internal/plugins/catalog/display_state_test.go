package catalog

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestProjectDisplayStateCoversFormalLifecycleEnum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		snapshot plugins.Snapshot
		want     string
	}{
		{
			name: "discovered",
			snapshot: plugins.Snapshot{
				Valid:             true,
				RegistrationState: plugins.RegistrationStateInstalled,
				DesiredState:      plugins.DesiredStateDisabled,
				RuntimeState:      plugins.RuntimeStateStopped,
				DisplayState:      plugins.DisplayStateDiscovered,
			},
			want: plugins.DisplayStateDiscovered,
		},
		{
			name: "invalid manifest",
			snapshot: plugins.Snapshot{
				Valid:             false,
				RegistrationState: plugins.RegistrationStateInstalled,
				DisplayState:      plugins.DisplayStateInvalidManifest,
			},
			want: plugins.DisplayStateInvalidManifest,
		},
		{
			name: "unknown display state",
			snapshot: plugins.Snapshot{
				Valid:             true,
				RegistrationState: plugins.RegistrationStateInstalled,
				DisplayState:      "paused",
			},
			want: plugins.DisplayStateInvalidManifest,
		},
		{
			name: "conflict",
			snapshot: plugins.Snapshot{
				Valid:             false,
				RegistrationState: plugins.RegistrationStateInstalled,
				DisplayState:      plugins.DisplayStateConflict,
			},
			want: plugins.DisplayStateConflict,
		},
		{
			name: "removed",
			snapshot: plugins.Snapshot{
				Valid:             true,
				RegistrationState: plugins.RegistrationStateRemoved,
			},
			want: "removed",
		},
		{
			name: "enabled",
			snapshot: plugins.Snapshot{
				Valid:             true,
				RegistrationState: plugins.RegistrationStateInstalled,
				DesiredState:      plugins.DesiredStateEnabled,
				RuntimeState:      plugins.RuntimeStateStopped,
			},
			want: "enabled",
		},
		{
			name: "enabling",
			snapshot: plugins.Snapshot{
				Valid:             true,
				RegistrationState: plugins.RegistrationStateInstalled,
				DesiredState:      plugins.DesiredStateEnabled,
				RuntimeState:      "starting",
			},
			want: "enabling",
		},
		{
			name: "running",
			snapshot: plugins.Snapshot{
				Valid:             true,
				RegistrationState: plugins.RegistrationStateInstalled,
				DesiredState:      plugins.DesiredStateEnabled,
				RuntimeState:      "running",
			},
			want: "running",
		},
		{
			name: "disabling",
			snapshot: plugins.Snapshot{
				Valid:             true,
				RegistrationState: plugins.RegistrationStateInstalled,
				DesiredState:      plugins.DesiredStateDisabled,
				RuntimeState:      "stopping",
			},
			want: "disabling",
		},
		{
			name: "stopping",
			snapshot: plugins.Snapshot{
				Valid:             true,
				RegistrationState: plugins.RegistrationStateInstalled,
				DesiredState:      plugins.DesiredStateEnabled,
				RuntimeState:      "stopping",
			},
			want: "stopping",
		},
		{
			name: "crashed",
			snapshot: plugins.Snapshot{
				Valid:             true,
				RegistrationState: plugins.RegistrationStateInstalled,
				DesiredState:      plugins.DesiredStateEnabled,
				RuntimeState:      "crashed",
			},
			want: "crashed",
		},
		{
			name: "backoff",
			snapshot: plugins.Snapshot{
				Valid:             true,
				RegistrationState: plugins.RegistrationStateInstalled,
				DesiredState:      plugins.DesiredStateEnabled,
				RuntimeState:      "backoff",
			},
			want: "backoff",
		},
		{
			name: "dead letter",
			snapshot: plugins.Snapshot{
				Valid:             true,
				RegistrationState: plugins.RegistrationStateInstalled,
				DesiredState:      plugins.DesiredStateEnabled,
				RuntimeState:      "dead_letter",
			},
			want: "dead_letter",
		},
		{
			name: "disabled",
			snapshot: plugins.Snapshot{
				Valid:             true,
				RegistrationState: plugins.RegistrationStateInstalled,
				DesiredState:      plugins.DesiredStateDisabled,
				RuntimeState:      plugins.RuntimeStateStopped,
			},
			want: "disabled",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := plugins.CloneSnapshot(tt.snapshot).DisplayState; got != tt.want {
				t.Fatalf("CloneSnapshot().DisplayState = %q, want %q", got, tt.want)
			}
		})
	}
}

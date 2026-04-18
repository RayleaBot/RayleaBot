package app

import (
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
)

func TestProjectServiceStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		systemStatus   string
		readinessState string
		want           string
	}{
		{name: "ready becomes running", readinessState: "ready", want: "running"},
		{name: "degraded stays degraded", readinessState: "degraded", want: "degraded"},
		{name: "failed stays failed", readinessState: "failed", want: "failed"},
		{name: "setup required stays setup required", readinessState: "setup_required", want: "setup_required"},
		{name: "shutdown overrides readiness", systemStatus: "shutting_down", readinessState: "ready", want: "stopping"},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := projectServiceStatus(tt.systemStatus, tt.readinessState); got != tt.want {
				t.Fatalf("projectServiceStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServiceStatusPayload(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		system    string
		readiness health.ReadinessReport
		want      map[string]any
	}{
		{
			name:   "running payload uses stable running summary",
			system: "running",
			readiness: health.ReadinessReport{
				Status: "ready",
			},
			want: map[string]any{
				"service_status": "running",
				"summary":        "服务运行中",
			},
		},
		{
			name:   "degraded payload keeps readiness reason and codes",
			system: "running",
			readiness: health.ReadinessReport{
				Status:      "degraded",
				Reason:      "OneBot 正在建立连接",
				ReasonCodes: []string{"adapter.connection_pending"},
			},
			want: map[string]any{
				"service_status": "degraded",
				"summary":        "服务运行条件受限",
				"reason":         "OneBot 正在建立连接",
				"reason_codes":   []string{"adapter.connection_pending"},
			},
		},
		{
			name:   "shutdown payload projects stopping summary",
			system: "shutting_down",
			readiness: health.ReadinessReport{
				Status: "ready",
			},
			want: map[string]any{
				"service_status": "stopping",
				"summary":        "服务正在停止",
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := serviceStatusPayload(tt.system, tt.readiness); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("serviceStatusPayload() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

package events

import (
	"reflect"
	"testing"

	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
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

			if got := ProjectServiceStatus(tt.systemStatus, tt.readinessState); got != tt.want {
				t.Fatalf("ProjectServiceStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServiceStatusPayload(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		system    string
		readiness systemsvc.ReadinessReport
		want      ServiceStatusPayload
	}{
		{
			name:   "running payload uses stable running summary",
			system: "running",
			readiness: systemsvc.ReadinessReport{
				Status: "ready",
			},
			want: ServiceStatusPayload{
				ServiceStatus: "running",
				Summary:       "服务运行中",
			},
		},
		{
			name:   "degraded payload keeps readiness reason and codes",
			system: "running",
			readiness: systemsvc.ReadinessReport{
				Status:      "degraded",
				Reason:      "OneBot 正在建立连接",
				ReasonCodes: []string{"adapter.connection_pending"},
			},
			want: ServiceStatusPayload{
				ServiceStatus: "degraded",
				Summary:       "服务运行条件受限",
				Reason:        "OneBot 正在建立连接",
				ReasonCodes:   []string{"adapter.connection_pending"},
			},
		},
		{
			name:   "shutdown payload projects stopping summary",
			system: "shutting_down",
			readiness: systemsvc.ReadinessReport{
				Status: "ready",
			},
			want: ServiceStatusPayload{
				ServiceStatus: "stopping",
				Summary:       "服务正在停止",
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ServiceStatusPayloadFrom(tt.system, tt.readiness); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ServiceStatusPayloadFrom() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// 发布同步完成，使用非阻塞读取可以直接断言没有多余帧。
func assertNoServiceStatusFrame(t *testing.T, updates <-chan Frame) {
	t.Helper()
	select {
	case frame := <-updates:
		t.Fatalf("unexpected service status frame: %#v", frame)
	default:
	}
}

func receiveServiceStatus(t *testing.T, updates <-chan Frame) ServiceStatusPayload {
	t.Helper()
	select {
	case frame := <-updates:
		return frame.Data.(ServiceStatusPayload)
	default:
		t.Fatal("missing service status frame")
		return ServiceStatusPayload{}
	}
}

type statusTestProvider struct {
	readiness      systemsvc.ReadinessReport
	readinessCalls int
}

func (p *statusTestProvider) SystemStatus() string { return "running" }
func (p *statusTestProvider) CurrentReadiness() systemsvc.ReadinessReport {
	p.readinessCalls++
	return p.readiness
}

func TestPublishServiceStatusOnlyWhenPayloadChanges(t *testing.T) {
	t.Parallel()
	provider := &statusTestProvider{readiness: systemsvc.ReadinessReport{Status: "ready"}}
	service := NewServiceStatusService(provider)
	updates, unsubscribe := service.Subscribe(4)
	defer unsubscribe()

	service.PublishSnapshot()
	if got := receiveServiceStatus(t, updates); got.ServiceStatus != "running" {
		t.Fatalf("initial payload = %#v", got)
	}
	service.PublishSnapshot()
	assertNoServiceStatusFrame(t, updates)

	for _, readiness := range []systemsvc.ReadinessReport{
		{Status: "degraded"},
		{Status: "degraded", Reason: "运行环境缺失"},
		{Status: "degraded", Reason: "运行环境缺失", ReasonCodes: []string{"platform.resource_missing"}},
		{Status: "degraded", Reason: "运行环境缺失", ReasonCodes: []string{}},
		{Status: "ready"},
	} {
		provider.readiness = readiness
		service.PublishSnapshot()
		got := receiveServiceStatus(t, updates)
		want := ServiceStatusPayloadFrom("running", readiness)
		want.ReasonCodes = append([]string{}, want.ReasonCodes...)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("payload = %#v, want %#v", got, want)
		}
		service.PublishSnapshot()
		assertNoServiceStatusFrame(t, updates)
	}
}

func TestServiceStatusSubscriptionGetsCurrentSnapshotWithoutChangingBroadcastBaseline(t *testing.T) {
	t.Parallel()
	provider := &statusTestProvider{readiness: systemsvc.ReadinessReport{Status: "ready"}}
	service := NewServiceStatusService(provider)
	service.PublishSnapshot()
	initial, existing, unsubscribe := service.SnapshotAndSubscribe(2)
	defer unsubscribe()
	if initial.Data.(ServiceStatusPayload).ServiceStatus != "running" {
		t.Fatalf("initial snapshot = %#v", initial)
	}
	service.PublishSnapshot()
	assertNoServiceStatusFrame(t, existing)

	provider.readiness = systemsvc.ReadinessReport{Status: "degraded", ReasonCodes: []string{"platform.resource_missing"}}
	current, newcomer, unsubscribeNew := service.SnapshotAndSubscribe(2)
	defer unsubscribeNew()
	payload := current.Data.(ServiceStatusPayload)
	if payload.ServiceStatus != "degraded" {
		t.Fatalf("new subscriber received stale snapshot: %#v", current)
	}
	payload.ReasonCodes[0] = "subscriber mutation"
	service.PublishSnapshot()
	first := receiveServiceStatus(t, existing)
	first.ReasonCodes[0] = "another subscriber mutation"
	second := receiveServiceStatus(t, newcomer)
	if second.ReasonCodes[0] != "platform.resource_missing" {
		t.Fatalf("subscribers share reason codes: %#v", second)
	}
	service.PublishSnapshot()
	assertNoServiceStatusFrame(t, existing)
	assertNoServiceStatusFrame(t, newcomer)
}

func TestPublishServiceStatusWhenReadinessChecksChange(t *testing.T) {
	t.Parallel()
	provider := &statusTestProvider{readiness: systemsvc.ReadinessReport{
		Status: "ready",
		Checks: map[string]string{"database": "ok", "runtime": "preparing"},
	}}
	service := NewServiceStatusService(provider)
	updates, unsubscribe := service.Subscribe(4)
	defer unsubscribe()
	publish := func() {
		t.Helper()
		calls := provider.readinessCalls
		service.PublishSnapshot()
		if got := provider.readinessCalls - calls; got != 1 {
			t.Fatalf("readiness evaluations per publish = %d, want 1", got)
		}
	}

	publish()
	initial := receiveServiceStatus(t, updates)
	assertNoServiceStatusFrame(t, updates)

	// 原地修改来源 map，不能同时改变上一份广播指纹。
	provider.readiness.Checks["runtime"] = "ok"
	publish()
	if got := receiveServiceStatus(t, updates); !reflect.DeepEqual(got, initial) {
		t.Fatalf("check-only change altered wire payload: %#v, want %#v", got, initial)
	}
	assertNoServiceStatusFrame(t, updates)

	publish()
	assertNoServiceStatusFrame(t, updates)

	// 不同 map 实例只要内容相同，就不应重复广播。
	provider.readiness.Checks = map[string]string{"runtime": "ok", "database": "ok"}
	publish()
	assertNoServiceStatusFrame(t, updates)
}

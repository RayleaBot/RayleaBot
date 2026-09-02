package lifecycle

import (
	"log/slog"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

// TestBackoffRestartStartFailureResetsManagerToStopped 回归线上故障：崩溃退避重启在
// 构建启动输入阶段失败（如 artifact 校验失败）时 Manager.Start 尚未执行，manager
// 会停留在 backoff 状态，之后每次 scheduler 触发都被视为等待重试而跳过启动，
// 插件永远无法自动恢复。失败路径必须把 manager 重置为 stopped，
// 让下一次触发或管理操作能再次尝试启动。
func TestBackoffRestartStartFailureResetsManagerToStopped(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(discardWriter{}, nil))
	application := newTestAppState(config.Config{}, logger)

	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "broken-artifact",
		Valid:             false, // BuildSpec 立即失败，等价于 artifact 校验失败
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "backoff",
	}})
	dispatcher := dispatch.New(logger, nil, nil, 16)
	runtimes := newRuntimeRegistry(logger, pluginruntime.Options{})
	manager := runtimes.GetOrCreate("broken-artifact")
	if manager == nil {
		t.Fatal("expected runtime manager")
	}
	manager.SetBackoffState(time.Now())

	application.setTestLifecycle(catalog, nil, runtimes, dispatcher, nil, nil, nil)

	application.services.pluginLifecycle.backoffRestart("broken-artifact", 0)

	if got := manager.Snapshot().State; got != pluginruntime.StateStopped {
		t.Fatalf("manager state = %q, want %q after failed backoff restart", got, pluginruntime.StateStopped)
	}
	if got, ok := catalog.Get("broken-artifact"); !ok || got.RuntimeState != string(pluginruntime.StateStopped) {
		t.Fatalf("catalog runtime_state = %q ok=%v, want stopped", got.RuntimeState, ok)
	}
}

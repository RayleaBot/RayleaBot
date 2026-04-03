// @vitest-environment jsdom
import { render, screen, within } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";
import { AppShell } from "@renderer/AppShell";
import type { LauncherSnapshot } from "@shared/launcher-models";

const snapshot: LauncherSnapshot = {
  settings: {
    installationRoot: "C:\\RayleaBot",
    closeBehavior: "ask_every_time",
  },
  resolvedSettings: {
    installationRoot: "C:\\RayleaBot",
    serverExecutablePath: "C:\\RayleaBot\\raylea-server.exe",
    configPath: "C:\\RayleaBot\\config\\user.yaml",
    workdir: "C:\\RayleaBot",
  },
  endpoint: {
    host: "127.0.0.1",
    port: 8080,
    baseUrl: "http://127.0.0.1:8080/",
  },
  environmentChecks: [
    {
      code: "config.bootstrap_available",
      title: "用户配置",
      severity: "warning",
      summary: "首次启动时会自动生成用户配置。",
      detail: "缺少用户配置文件。",
      remediation: "启动服务后会基于 default.yaml 生成首份用户配置。",
    },
  ],
  recentStderr: ["stderr line"],
  processId: null,
  serviceState: "stopped",
  serviceOwnership: "none",
  shutdownRequested: false,
  serviceDetail: "服务尚未启动。",
  lastError: "",
  releaseCheck: {
    status: "up_to_date",
    currentVersion: "0.1.0",
    latestVersion: "0.1.0",
    summary: "当前版本 0.1.0 已是最新。",
    detail: "",
    releasePageUrl: "https://example.invalid/releases/v0.1.0",
    updateAvailable: false,
  },
  recoverySummary: {
    status: "degraded",
    phase: "post_startup",
    operation: "upgrade",
    created_at: "2026-04-02T08:00:00Z",
    updated_at: "2026-04-02T08:01:00Z",
    issues: [
      {
        code: "recovery.plugin_min_core_version",
        severity: "warning",
        summary: "插件 weather-pro 需要更高版本的 RayleaBot core。",
        remediation: "升级程序或重新安装兼容插件。",
      },
    ],
    skipped_plugins: [
      {
        plugin_id: "weather-pro",
        reason_code: "plugin.min_core_version",
        summary: "插件最低 core 版本要求不满足。",
      },
    ],
    manual_actions: ["处理被跳过插件的兼容性问题后，再在管理面中手动重新启用。"],
    next_steps: ["查看管理面中的恢复摘要并处理跳过插件。", "通过管理面、Launcher 或 diagnostics 复核 recovery_summary。"],
  },
};

function renderStatusShell() {
  return render(
    <AppShell
      snapshot={snapshot}
      activeSection="status"
      platformLabel="win32-x64"
      settingsDraft={snapshot.settings}
      resolvedSettings={snapshot.resolvedSettings}
      editingSettings={false}
      diagnosticsSummary=""
      busyAction={null}
      controlsDisabled={false}
      isMaximized={false}
      onNavigate={vi.fn()}
      onRefresh={vi.fn()}
      onStart={vi.fn()}
      onStop={vi.fn()}
      onOpenWeb={vi.fn()}
      onRecoveryRecheck={vi.fn()}
      onRuntimeBootstrap={vi.fn()}
      onOpenRecoveryPlugin={vi.fn()}
      onOpenReleasePage={vi.fn()}
      onOpenLogs={vi.fn()}
      onResetAdmin={vi.fn()}
      onBeginEdit={vi.fn()}
      onCancelEdit={vi.fn()}
      onSaveSettings={vi.fn()}
      onUpdateInstallationRoot={vi.fn()}
      onUpdateCloseBehavior={vi.fn()}
      onUpdateAdvancedOverride={vi.fn()}
      onChooseInstallationRoot={vi.fn()}
      onChooseServer={vi.fn()}
      onChooseConfig={vi.fn()}
      onChooseWorkdir={vi.fn()}
      onExit={vi.fn()}
    />,
  );
}

describe("AppShell", () => {
  test("renders navigation, hero summary, and environment warning", () => {
    render(
      <AppShell
        snapshot={snapshot}
        activeSection="status"
        platformLabel="win32-x64"
        settingsDraft={snapshot.settings}
        resolvedSettings={snapshot.resolvedSettings}
        editingSettings={false}
        diagnosticsSummary=""
        busyAction={null}
        controlsDisabled={false}
        isMaximized={false}
        onNavigate={vi.fn()}
        onRefresh={vi.fn()}
        onStart={vi.fn()}
        onStop={vi.fn()}
        onOpenWeb={vi.fn()}
        onRecoveryRecheck={vi.fn()}
        onRuntimeBootstrap={vi.fn()}
        onOpenRecoveryPlugin={vi.fn()}
        onOpenReleasePage={vi.fn()}
        onOpenLogs={vi.fn()}
        onResetAdmin={vi.fn()}
        onBeginEdit={vi.fn()}
        onCancelEdit={vi.fn()}
        onSaveSettings={vi.fn()}
        onUpdateInstallationRoot={vi.fn()}
        onUpdateCloseBehavior={vi.fn()}
        onUpdateAdvancedOverride={vi.fn()}
        onChooseInstallationRoot={vi.fn()}
        onChooseServer={vi.fn()}
        onChooseConfig={vi.fn()}
        onChooseWorkdir={vi.fn()}
        onExit={vi.fn()}
      />,
    );

    expect(screen.getByText("RayleaBot")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "RayleaLauncher" })).toBeInTheDocument();
    expect(screen.getByText("运行状态")).toBeInTheDocument();
    expect(screen.getByText("环境检查")).toBeInTheDocument();
    expect(screen.getByText("日志诊断")).toBeInTheDocument();
    expect(screen.getByText("偏好设置")).toBeInTheDocument();
    expect(screen.getByText("服务尚未启动。")).toBeInTheDocument();
    expect(screen.getByText("首次启动时会自动生成用户配置。")).toBeInTheDocument();
    expect(screen.getByText("警告")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "处理建议" })).not.toBeInTheDocument();
    expect(screen.getByText(/恢复兼容性/)).toBeInTheDocument();
    expect(screen.getByText("degraded · upgrade")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "重新检查" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "准备运行时" })).toBeInTheDocument();
  });

  test("restores editing controls and advanced overrides in settings", () => {
    const { container } = render(
      <AppShell
        snapshot={snapshot}
        activeSection="settings"
        platformLabel="win32-x64"
        settingsDraft={{
          ...snapshot.settings,
          advancedOverrides: {
            serverExecutablePath: "D:\\Portable\\server\\raylea-server.exe",
            configPath: "D:\\Portable\\config\\user.yaml",
            workdir: "D:\\Portable",
          },
        }}
        resolvedSettings={snapshot.resolvedSettings}
        editingSettings={true}
        diagnosticsSummary=""
        busyAction={null}
        controlsDisabled={false}
        isMaximized={false}
        onNavigate={vi.fn()}
        onRefresh={vi.fn()}
        onStart={vi.fn()}
        onStop={vi.fn()}
        onOpenWeb={vi.fn()}
        onRecoveryRecheck={vi.fn()}
        onRuntimeBootstrap={vi.fn()}
        onOpenRecoveryPlugin={vi.fn()}
        onOpenReleasePage={vi.fn()}
        onOpenLogs={vi.fn()}
        onResetAdmin={vi.fn()}
        onBeginEdit={vi.fn()}
        onCancelEdit={vi.fn()}
        onSaveSettings={vi.fn()}
        onUpdateInstallationRoot={vi.fn()}
        onUpdateCloseBehavior={vi.fn()}
        onUpdateAdvancedOverride={vi.fn()}
        onChooseInstallationRoot={vi.fn()}
        onChooseServer={vi.fn()}
        onChooseConfig={vi.fn()}
        onChooseWorkdir={vi.fn()}
        onExit={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "放弃" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "收起高级覆盖" })).toBeInTheDocument();
    expect(screen.getByText("服务端覆盖")).toBeInTheDocument();
    expect(screen.getByText("配置覆盖")).toBeInTheDocument();
    expect(screen.getByText("运行目录覆盖")).toBeInTheDocument();
    expect(screen.getByText("当前解析结果")).toBeInTheDocument();
    expect(screen.getByText("当前生效的服务端、配置与工作目录路径。")).toBeInTheDocument();
    expect(screen.getByText("路径变更尚未保存，当前显示的是预览结果。")).toBeInTheDocument();
    expect(screen.getByText("关闭窗口时采用的默认动作。托盘模式会保留后台入口。")).toBeInTheDocument();
    expect(screen.getByText("每次关闭窗口时都显示确认选项。")).toBeInTheDocument();
    expect(screen.getByText("关闭主窗口后保留托盘入口和后台状态。")).toBeInTheDocument();
    expect(screen.getByText("直接结束启动器窗口与托盘进程。")).toBeInTheDocument();
    expect(screen.getByText("维护操作")).toBeInTheDocument();
    expect(screen.getByText("清除本地管理凭据，下次启动时重新完成初始化。")).toBeInTheDocument();
    expect(screen.getByText("关闭窗口和托盘入口，不影响已保存配置与服务文件。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "立即重置" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "退出启动器" })).toBeInTheDocument();
    expect(container.querySelector(".settings-edit-bar")).not.toBeNull();
    expect(container.querySelector(".maintenance-action-card")).not.toBeNull();
    expect(container.querySelector(".settings-resolution-panel")).not.toBeNull();
    expect(container.querySelector(".settings-info-card")).toBeNull();
  });

  test("renders the balanced status homepage layout", () => {
    const { container } = renderStatusShell();

    expect(container.querySelector(".status-homepage")).not.toBeNull();
    expect(container.querySelector(".status-hero")).not.toBeNull();
    expect(container.querySelector(".status-hero__body")).not.toBeNull();
    expect(container.querySelector(".status-hero__actions")).not.toBeNull();
    expect(container.querySelector(".status-summary-grid")).not.toBeNull();
    expect(container.querySelector(".status-summary-rail")).not.toBeNull();
    expect(container.querySelector(".status-log-panel")).not.toBeNull();

    const primaryAction = screen.getByRole("button", { name: "启动 RayleaBot" });
    const stopAction = screen.getByRole("button", { name: "停止服务" });
    const manageAction = screen.getByRole("button", { name: "管理面板" });

    expect(primaryAction.closest(".status-hero__primary-action")).not.toBeNull();
    expect(stopAction.closest(".status-hero__secondary-actions")).not.toBeNull();
    expect(manageAction.closest(".status-hero__secondary-actions")).not.toBeNull();

    const rail = container.querySelector(".status-summary-rail");
    expect(rail).not.toBeNull();
    expect(within(rail as HTMLElement).getByText("版本监控")).toBeInTheDocument();
    expect(within(rail as HTMLElement).getByText("恢复兼容性")).toBeInTheDocument();
    expect(within(rail as HTMLElement).getByText("环境预警")).toBeInTheDocument();
  });
});

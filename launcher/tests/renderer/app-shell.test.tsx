// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";
import { AppShell } from "@renderer/AppShell";
import type { LauncherSnapshot } from "@shared/launcher-models";
import type { ComponentProps } from "react";

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
        review_id: "review_weather_pro",
        review_status: "pending",
      },
    ],
    manual_actions: ["处理被跳过插件的兼容性问题后，再在管理面中手动重新启用。"],
    next_steps: ["查看管理面中的恢复摘要并处理跳过插件。", "通过管理面、Launcher 或 diagnostics 复核 recovery_summary。"],
  },
};

function renderShell(overrides: Partial<ComponentProps<typeof AppShell>> = {}) {
  return render(
    <AppShell
      snapshot={snapshot}
      activeSection="status"
      renderedSection="status"
      sectionTransitionState="idle"
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
      {...overrides}
    />,
  );
}

describe("AppShell", () => {
  test("renders the shared section header with title, summary, and action", () => {
    const { container } = renderShell();

    expect(screen.getByRole("heading", { name: "运行状态" })).toBeInTheDocument();
    expect(screen.getByText("查看当前服务状态，并直接处理启动、停止、管理和恢复动作。")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "刷新状态" })).toHaveLength(2);
    expect(container.querySelector(".section-shell")).not.toBeNull();
    expect(container.querySelector(".section-header")).not.toBeNull();
  });

  test("renders navigation, hero summary, and ordered status rail", () => {
    const { container } = renderShell();

    expect(screen.getByText("RayleaBot")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "RayleaLauncher" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "运行状态" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "环境检查" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "日志诊断" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "偏好设置" })).toBeInTheDocument();
    expect(screen.getByText("首次启动时会自动生成用户配置。")).toBeInTheDocument();
    expect(screen.getByText("degraded · upgrade")).toBeInTheDocument();
    expect(container.querySelector(".status-homepage")).not.toBeNull();
    expect(container.querySelector(".status-hero")).not.toBeNull();
    expect(container.querySelector(".status-action-feedback")).not.toBeNull();
    expect(screen.getByText("当前没有进行中的操作。")).toBeInTheDocument();

    const rail = container.querySelector(".status-summary-rail");
    expect(rail).not.toBeNull();
    const railTitles = Array.from(rail?.querySelectorAll(".brand-eyebrow--tight") ?? []).map((node) => node.textContent);
    expect(railTitles).toEqual(["环境预警", "恢复兼容性", "版本监控"]);

    const primaryAction = screen.getByRole("button", { name: "启动 RayleaBot" });
    expect(primaryAction.closest(".status-hero__primary-action")).not.toBeNull();
    expect(screen.getByRole("button", { name: "停止服务" }).closest(".status-hero__secondary-actions")).not.toBeNull();
    expect(screen.getByRole("button", { name: "管理面板" }).closest(".status-hero__secondary-actions")).not.toBeNull();
  });

  test("shows the constrained reason on the status page when readiness is degraded", () => {
    renderShell({
      snapshot: {
        ...snapshot,
        serviceState: "degraded",
        serviceOwnership: "launcher_managed",
        serviceDetail: "Python 运行环境尚未准备完成。",
        environmentChecks: [
          {
            code: "runtime.python_managed_ready",
            title: "Python 运行环境准备",
            severity: "warning",
            summary: "依赖 Python 运行环境的功能暂不可用。",
            detail: "当前平台的 Python 运行环境缺少本地可用资源。",
            remediation: "请联网准备运行环境，或按正式目录结构手动预置资源。",
          },
        ],
      },
    });

    expect(screen.getAllByText("运行条件受限").length).toBeGreaterThan(0);
    expect(screen.getByText("当前限制")).toBeInTheDocument();
    expect(screen.getAllByText("Python 运行环境尚未准备完成。").length).toBeGreaterThan(0);
    expect(screen.getByText("处理提示")).toBeInTheDocument();
    expect(screen.getByText(/按正式目录结构手动预置资源/)).toBeInTheDocument();
  });

  test("shows startup preparation detail instead of constrained wording while the service is starting", () => {
    renderShell({
      snapshot: {
        ...snapshot,
        serviceState: "starting",
        serviceOwnership: "launcher_managed",
        serviceDetail: "正在准备运行环境并等待服务就绪。",
      },
    });

    expect(screen.getAllByText("启动中").length).toBeGreaterThan(0);
    expect(screen.getByText("运行说明")).toBeInTheDocument();
    expect(screen.getAllByText("正在准备运行环境并等待服务就绪。").length).toBeGreaterThan(0);
  });

  test("disables recovery recheck when there is no recovery summary", () => {
    renderShell({
      snapshot: {
        ...snapshot,
        serviceState: "running",
        serviceOwnership: "launcher_managed",
        recoverySummary: null,
      },
    });

    const buttons = screen.getAllByRole("button", { name: "重新检查" });
    expect(buttons).toHaveLength(1);
    expect(buttons[0]).toBeDisabled();
    expect(screen.getByText("当前没有恢复摘要。")).toBeInTheDocument();
  });

  test("renders environment cards with summary detail and remediation blocks", () => {
    const { container } = renderShell({
      activeSection: "environment",
      renderedSection: "environment",
      snapshot: {
        ...snapshot,
        environmentChecks: [
          {
            code: "runtime.python_managed_ready",
            title: "Python 运行环境准备",
            severity: "ok",
            summary: "Python 运行环境已纳入启动流程。",
            detail: "当前平台的 Python 运行环境配置信息完整，启动服务时会自动准备。",
            remediation: "请联网准备运行环境；离线或受限网络环境可预置已校验归档到 C:\\RayleaBot\\cache\\downloads\\runtime\\python-runtime.tar.gz，或预展开到 C:\\RayleaBot\\.deps\\store\\python-runtime\\3.12.13。",
          },
        ],
      },
    });

    expect(screen.getByRole("heading", { name: "环境检查" })).toBeInTheDocument();
    expect(screen.getByText("Python 运行环境已纳入启动流程。")).toBeInTheDocument();
    expect(screen.getByText("当前平台的 Python 运行环境配置信息完整，启动服务时会自动准备。")).toBeInTheDocument();
    expect(screen.getByText("离线准备")).toBeInTheDocument();
    expect(screen.getByText(/预展开到 C:\\RayleaBot\\.deps\\store\\python-runtime\\3.12.13/)).toBeInTheDocument();
    expect(container.querySelector(".check-item__remediation")).not.toBeNull();
  });

  test("renders draft and resolved settings surfaces during editing", () => {
    const { container } = renderShell({
      activeSection: "settings",
      renderedSection: "settings",
      editingSettings: true,
      settingsDraft: {
        ...snapshot.settings,
        advancedOverrides: {
          serverExecutablePath: "D:\\Portable\\server\\raylea-server.exe",
          configPath: "D:\\Portable\\config\\user.yaml",
          workdir: "D:\\Portable",
        },
      },
    });

    expect(screen.getByRole("heading", { name: "偏好设置" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "放弃" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存" })).toBeInTheDocument();
    expect(screen.getByText("当前显示草稿路径与预览结果，保存后才会切换为生效值。")).toBeInTheDocument();
    expect(screen.getAllByText("当前草稿").length).toBeGreaterThan(0);
    expect(screen.getAllByText("当前生效").length).toBeGreaterThan(0);
    expect(screen.getByText("服务端覆盖")).toBeInTheDocument();
    expect(screen.getByText("配置覆盖")).toBeInTheDocument();
    expect(screen.getByText("运行目录覆盖")).toBeInTheDocument();
    expect(container.querySelector(".settings-compare-strip")).not.toBeNull();
    expect(container.querySelector(".settings-resolution-panel")).not.toBeNull();
    expect(container.querySelector(".settings-edit-bar")).not.toBeNull();
  });

  test("renders quiet diagnostics state without error styling when stderr is empty", () => {
    renderShell({
      activeSection: "diagnostics",
      renderedSection: "diagnostics",
      diagnosticsSummary: "服务状态：稳定",
      snapshot: {
        ...snapshot,
        recentStderr: [],
      },
    });

    expect(screen.getByRole("heading", { name: "日志诊断" })).toBeInTheDocument();
    expect(screen.getByText("当前没有新的异常输出。")).toBeInTheDocument();
    expect(screen.getByText("诊断摘要已准备好，当前输出平稳。")).toBeInTheDocument();
  });

  test("marks current and rendered section metadata for transitions", () => {
    const { container } = renderShell({
      activeSection: "environment",
      renderedSection: "status",
      sectionTransitionState: "exiting",
    });

    const shellMain = container.querySelector(".shell-main");
    expect(shellMain?.getAttribute("data-active-section")).toBe("environment");
    expect(shellMain?.getAttribute("data-rendered-section")).toBe("status");
    expect(shellMain?.getAttribute("data-transition")).toBe("exiting");
  });
});

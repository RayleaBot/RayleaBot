// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";
import { AppShell } from "@renderer/AppShell";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { createLauncherSnapshot } from "../helpers/snapshot";
import type { ComponentProps } from "react";

const snapshot: LauncherSnapshot = createLauncherSnapshot({
  launcher: {
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
        scope: "preflight",
        code: "config.bootstrap_available",
        title: "用户配置",
        severity: "warning",
        summary: "首次启动时会自动生成用户配置。",
        detail: "缺少用户配置文件。",
        remediation: "启动服务后会基于 default.yaml 生成首份用户配置。",
      },
    ],
    preflightChecks: [
      {
        scope: "preflight",
        code: "config.bootstrap_available",
        title: "用户配置",
        severity: "warning",
        summary: "首次启动时会自动生成用户配置。",
        detail: "缺少用户配置文件。",
        remediation: "启动服务后会基于 default.yaml 生成首份用户配置。",
      },
    ],
    recentStderr: ["stderr line"],
    releaseCheck: {
      status: "up_to_date",
      currentVersion: "0.1.0",
      latestVersion: "0.1.0",
      summary: "当前版本 0.1.0 已是最新。",
      detail: "",
      releasePageUrl: "https://example.invalid/releases/v0.1.0",
      updateAvailable: false,
      canCheck: true,
    },
    localRecoverySummary: {
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
      manual_actions: ["处理被跳过插件的兼容性问题后，再在管理界面中手动重新启用。"],
      next_steps: ["查看管理界面中的恢复摘要并处理跳过插件。", "通过管理界面、Launcher 或 diagnostics 复核 recovery_summary。"],
    },
  },
});

function renderShell(overrides: Partial<ComponentProps<typeof AppShell>> = {}) {
  return render(
    <AppShell
      snapshot={snapshot}
      activeSection="status"
      renderedSection="status"
      sectionTransitionState="idle"
      platformLabel="win32-x64"
      settingsDraft={snapshot.launcher.settings}
      resolvedSettings={snapshot.launcher.resolvedSettings}
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
      onOpenRecoveryTasks={vi.fn()}
      onOpenRuntimeTasks={vi.fn()}
      onOpenRecoveryPlugin={vi.fn()}
      onCheckForUpdates={vi.fn()}
      onDownloadUpdate={vi.fn()}
      onInstallDownloadedUpdate={vi.fn()}
      onOpenRepositoryPage={vi.fn()}
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
  test("renders the shared section header with title and action", () => {
    const { container } = renderShell();

    expect(screen.getByRole("heading", { name: "运行状态" })).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "刷新状态" })).toHaveLength(1);
    expect(container.querySelector(".section-shell")).not.toBeNull();
    expect(container.querySelector(".section-header")).not.toBeNull();
  });

  test("renders navigation, hero summary, and ordered status rail", () => {
    const { container } = renderShell();

    expect(screen.getByRole("button", { name: "运行状态" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "环境检查" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "日志诊断" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "偏好设置" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "关于应用" })).toBeInTheDocument();
    expect(screen.getByText("首次启动时会自动生成用户配置。")).toBeInTheDocument();
    expect(screen.getByText("部分受限 · 升级 · 启动后")).toBeInTheDocument();
    expect(container.querySelector(".status-homepage")).not.toBeNull();
    expect(container.querySelector(".status-hero")).not.toBeNull();
    expect(container.querySelector(".status-action-feedback")).not.toBeNull();
    expect(screen.getByText("当前没有进行中的操作。")).toBeInTheDocument();

    const rail = container.querySelector(".status-summary-rail");
    expect(rail).not.toBeNull();
    const railTitles = Array.from(rail?.querySelectorAll(".brand-eyebrow--tight") ?? []).map((node) => node.textContent);
    expect(railTitles).toEqual(["环境预警", "恢复兼容性"]);

    const primaryAction = screen.getByRole("button", { name: "启动 RayleaBot" });
    expect(primaryAction.closest(".status-hero__primary-action")).not.toBeNull();
    expect(screen.getByRole("button", { name: "停止服务" }).closest(".status-hero__secondary-actions")).not.toBeNull();
    expect(screen.getByRole("button", { name: "管理界面" }).closest(".status-hero__secondary-actions")).not.toBeNull();
  });

  test("shows the constrained reason on the status page when readiness is degraded", () => {
    renderShell({
      snapshot: {
        ...snapshot,
        server: {
          ...snapshot.server,
          health: { status: "ok" },
          readiness: {
            status: "degraded",
            reason: "Python 运行环境元数据不完整。",
            reason_codes: ["platform.resource_missing"],
            checks: {
              runtime: "resource_missing",
            },
            issues: [
              {
                code: "platform.resource_missing",
                severity: "warning",
                summary: "Python 运行环境元数据不完整。",
                remediation: "请在 .deps/manifest.json 中补齐当前平台 Python 运行环境资源。",
              },
            ],
          },
        },
        launcher: {
          ...snapshot.launcher,
          processLifecycle: "running",
          processOwnership: "launcher_managed",
          environmentChecks: [
            {
              scope: "advisory",
              code: "runtime.python_managed_ready",
              title: "Python 运行环境准备",
              severity: "warning",
              summary: "依赖 Python 运行环境的功能暂不可用。",
              detail: "当前平台的 Python 运行环境缺少本地可用资源。",
              remediation: "启动运行环境任务准备 Python 依赖。",
            },
          ],
        },
      },
    });

    expect(screen.getAllByText("运行条件受限").length).toBeGreaterThan(0);
    expect(screen.getByText("当前限制")).toBeInTheDocument();
    expect(screen.getAllByText("Python 运行环境元数据不完整。")).toHaveLength(1);
    expect(screen.getAllByText(/请在 \.deps\/manifest\.json 中补齐当前平台 Python 运行环境资源。/)).toHaveLength(1);
    expect(screen.getByText("服务诊断")).toBeInTheDocument();
    expect(screen.getAllByText("platform.resource_missing").length).toBeGreaterThan(0);
  });

  test("shows startup preparation detail instead of constrained wording while the service is starting", () => {
    renderShell({
      snapshot: {
        ...snapshot,
        launcher: {
          ...snapshot.launcher,
          processLifecycle: "starting",
          processOwnership: "launcher_managed",
          statusHint: "正在准备运行环境并等待服务就绪。",
        },
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
        server: {
          ...snapshot.server,
          health: { status: "ok" },
          readiness: { status: "ready" },
        },
        launcher: {
          ...snapshot.launcher,
          processLifecycle: "running",
          processOwnership: "launcher_managed",
          localRecoverySummary: null,
        },
      },
    });

    const buttons = screen.getAllByRole("button", { name: "执行恢复检查" });
    expect(buttons).toHaveLength(1);
    expect(buttons[0]).toBeDisabled();
    expect(screen.getByText("没有恢复兼容性摘要。")).toBeInTheDocument();
  });

  test("renders preflight checks on the environment page without runtime resource cards", () => {
    const { container } = renderShell({
      activeSection: "environment",
      renderedSection: "environment",
      snapshot: {
        ...snapshot,
        launcher: {
          ...snapshot.launcher,
          environmentChecks: [
            {
              scope: "preflight",
              code: "workdir.unwritable",
              title: "工作目录",
              severity: "error",
              summary: "工作目录不可写。",
              detail: "工作目录写入失败。",
              remediation: "请先选择可写的工作目录，再启动服务。",
            },
          ],
          preflightChecks: [
            {
              scope: "preflight",
              code: "workdir.unwritable",
              title: "工作目录",
              severity: "error",
              summary: "工作目录不可写。",
              detail: "工作目录写入失败。",
              remediation: "请先选择可写的工作目录，再启动服务。",
            },
          ],
        },
      },
    });

    expect(screen.getByRole("heading", { name: "环境检查" })).toBeInTheDocument();
    expect(screen.getByText("工作目录不可写。")).toBeInTheDocument();
    expect(screen.getByText("工作目录写入失败。")).toBeInTheDocument();
    expect(screen.getByText("处理方式")).toBeInTheDocument();
    expect(screen.getByText("请先选择可写的工作目录，再启动服务。")).toBeInTheDocument();
    expect(container.querySelector(".check-item__remediation")).not.toBeNull();
  });

  test("uses refresh for the environment recheck action", () => {
    const onRefresh = vi.fn();
    renderShell({
      activeSection: "environment",
      renderedSection: "environment",
      onRefresh,
    });

    const recheckButton = screen.getByRole("button", { name: "重新检查" });
    expect(recheckButton).toBeEnabled();

    fireEvent.click(recheckButton);

    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  test("renders editable settings paths during editing", () => {
    const { container } = renderShell({
      activeSection: "settings",
      renderedSection: "settings",
      editingSettings: true,
      settingsDraft: {
        ...snapshot.launcher.settings,
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
    expect(screen.getByText("路径设置")).toBeInTheDocument();
    expect(screen.getByText("可编辑")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "服务端程序" })).toHaveValue("D:\\Portable\\server\\raylea-server.exe");
    expect(screen.getByRole("textbox", { name: "配置文件" })).toHaveValue("D:\\Portable\\config\\user.yaml");
    expect(screen.getByRole("textbox", { name: "进程工作目录" })).toHaveValue("D:\\Portable");
    expect(container.querySelector(".settings-paths-panel")).not.toBeNull();
    expect(container.querySelector(".settings-edit-bar")).not.toBeNull();
  });

  test("renders quiet diagnostics state without error styling when stderr is empty", () => {
    renderShell({
      activeSection: "diagnostics",
      renderedSection: "diagnostics",
      diagnosticsSummary: "服务状态：稳定",
      snapshot: {
        ...snapshot,
        launcher: {
          ...snapshot.launcher,
          recentStderr: [],
        },
      },
    });

    expect(screen.getByRole("heading", { name: "日志诊断" })).toBeInTheDocument();
    expect(screen.getByText("暂无异常日志")).toBeInTheDocument();
    expect(screen.getByText("未发现异常日志。")).toBeInTheDocument();
  });

  test("renders the about page with repository access", () => {
    const onCheckForUpdates = vi.fn();
    const onOpenRepositoryPage = vi.fn();
    const { container } = renderShell({
      activeSection: "about",
      renderedSection: "about",
      onCheckForUpdates,
      onOpenRepositoryPage,
    });

    expect(screen.getByRole("heading", { name: "关于应用" })).toBeInTheDocument();
    expect(screen.getByText("RayleaBot 启动器")).toBeInTheDocument();
    expect(screen.getByText("RayleaLauncher")).toBeInTheDocument();
    expect(screen.getByText("AGPL-3.0")).toBeInTheDocument();
    expect(container.querySelector(".about-update-panel")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "GitHub" }));
    fireEvent.click(screen.getByRole("button", { name: "检查更新" }));

    expect(onOpenRepositoryPage).toHaveBeenCalledTimes(1);
    expect(onCheckForUpdates).toHaveBeenCalledTimes(1);
  });

  test("shows update availability next to the current version", () => {
    renderShell({
      activeSection: "about",
      renderedSection: "about",
      snapshot: {
        ...snapshot,
        launcher: {
          ...snapshot.launcher,
          releaseCheck: {
            ...snapshot.launcher.releaseCheck,
            status: "update_available",
            latestVersion: "0.2.0",
            summary: "发现新版本 0.2.0。",
            updateAvailable: true,
            canDownload: true,
          },
        },
      },
    });

    expect(screen.getAllByText("0.1.0").length).toBeGreaterThan(0);
    expect(screen.getByText("有新版本 0.2.0")).toBeInTheDocument();
  });

  test("shows development version on the about page when release version is unavailable", () => {
    renderShell({
      activeSection: "about",
      renderedSection: "about",
      snapshot: {
        ...snapshot,
        launcher: {
          ...snapshot.launcher,
          releaseCheck: {
            ...snapshot.launcher.releaseCheck,
            currentVersion: "",
            latestVersion: "",
          },
        },
      },
    });

    expect(screen.getAllByText("开发").length).toBeGreaterThan(0);
    expect(screen.queryByText("0.1.0")).not.toBeInTheDocument();
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

// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";

import { AppShellDiagnosticsSection } from "@renderer/AppShellDiagnosticsSection";
import { AppShellEnvironmentSection } from "@renderer/AppShellEnvironmentSection";
import { AppShellAboutSection } from "@renderer/AppShellAboutSection";
import { AppShellSettingsSection } from "@renderer/AppShellSettingsSection";
import { AppShellStatusSection } from "@renderer/AppShellStatusSection";
import { isRuntimePreparationIssue } from "@renderer/AppShell.shared";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { createLauncherSnapshot } from "../helpers/snapshot";

const noop = vi.fn();

function renderStatusSection(snapshot: LauncherSnapshot, onOpenWeb = noop) {
  return render(
    <AppShellStatusSection
      snapshot={snapshot}
      resolvedSettings={snapshot.launcher.resolvedSettings}
      busyAction={null}
      controlsDisabled={false}
      onStart={noop}
      onStop={noop}
      onOpenWeb={onOpenWeb}
      onOpenLogs={noop}
    />,
  );
}

describe("runtime preparation issue classification", () => {
  test.each([
    ["deps.manifest_missing", true],
    ["chromium.not_ready", true],
    ["python.not_ready", false],
    ["nodejs.not_ready", false],
    ["npm.not_ready", false],
    ["config.user", false],
  ])("classifies %s", (code, expected) => {
    expect(isRuntimePreparationIssue(code)).toBe(expected);
  });
});

const configuredSnapshot = createLauncherSnapshot({
  launcher: {
    settings: {
      installationRoot: "C:\\RayleaBot",
      closeBehavior: "ask_every_time",
    },
    resolvedSettings: {
      installationRoot: "C:\\RayleaBot",
      serverExecutablePath: "C:\\RayleaBot\\server\\raylea-server.exe",
      configPath: "C:\\RayleaBot\\config\\user.yaml",
      workdir: "C:\\RayleaBot",
    },
  },
});

describe("Launcher workspace presentation", () => {
  test.each([
    ["", "尚未检查"],
    ["初始化失败。", "检查结果不可用"],
  ])("does not report empty checks as healthy when the local error is %j", (lastLocalError, label) => {
    render(<AppShellEnvironmentSection snapshot={createLauncherSnapshot({ launcher: { lastLocalError } })} platformLabel="Windows x64" />);
    expect(screen.getByRole("heading", { name: label })).toBeVisible();
    expect(screen.queryByText("可以启动")).not.toBeInTheDocument();
    expect(screen.queryByText("当前未发现阻塞或警告项。")).not.toBeInTheDocument();
  });

  test("shows issues immediately while keeping healthy environment checks collapsed", () => {
    const snapshot = createLauncherSnapshot({
      launcher: {
        preflightChecks: [
          {
            scope: "preflight",
            code: "config.user",
            title: "用户配置",
            severity: "error",
            summary: "配置文件不可读。",
            detail: "无法读取当前用户配置。",
            remediation: "重新选择有效的配置文件。",
          },
          {
            scope: "preflight",
            code: "server.executable",
            title: "服务端程序",
            severity: "ok",
            summary: "已找到可执行文件。",
            detail: "服务端程序可用。",
            remediation: "",
          },
        ],
      },
    });

    render(<AppShellEnvironmentSection snapshot={snapshot} platformLabel="Windows x64" />);

    expect(screen.getByText("配置文件不可读。")).toBeVisible();
    expect(screen.getByText("重新选择有效的配置文件。")).toBeVisible();
    const disclosure = screen.getByText("查看正常项").closest("details");
    expect(disclosure).not.toHaveAttribute("open");
    expect(within(disclosure as HTMLElement).getByText("服务端程序")).toBeInTheDocument();

    fireEvent.click(within(disclosure as HTMLElement).getByText("查看正常项"));
    expect(disclosure).toHaveAttribute("open");
  });

  test("explains a failed start with its hint and cause rather than a generic local error", () => {
    renderStatusSection(createLauncherSnapshot({
      launcher: {
        statusHint: "服务进程在启动阶段提前退出。",
        lastLocalError: "listen tcp 127.0.0.1:8080: bind: address already in use",
      },
    }));

    expect(screen.getByText("启动失败")).toBeInTheDocument();
    expect(screen.getByText("服务进程在启动阶段提前退出。")).toBeInTheDocument();
    expect(screen.getByText("失败原因")).toBeInTheDocument();
    expect(screen.getByText("listen tcp 127.0.0.1:8080: bind: address already in use")).toBeInTheDocument();
    expect(screen.queryByText("启动器检测到本地异常。")).not.toBeInTheDocument();
    expect(screen.queryByText("当前限制")).not.toBeInTheDocument();
  });

  test("sends runtime preparation to the management UI for the resources the service reports", () => {
    const onOpenWeb = vi.fn();
    renderStatusSection(createLauncherSnapshot({
      server: {
        health: { status: "ok" },
        readiness: {
          status: "degraded",
          reason: "图片渲染 Chromium 未准备。",
          reason_codes: ["platform.resource_missing"],
          checks: { render: "resource_missing" },
          issues: [{
            code: "platform.resource_missing",
            severity: "warning",
            summary: "图片渲染 Chromium 未准备。",
            remediation: "启动运行环境任务准备图片渲染 Chromium。",
            runtime_resources: ["chromium"],
          }],
        },
      },
      launcher: {
        processLifecycle: "running",
        processOwnership: "launcher_managed",
        preflightChecks: [{
          scope: "preflight",
          code: "chromium.not_ready",
          title: "图片渲染 Chromium",
          severity: "ok",
          summary: "尚未准备，启动服务时自动准备。",
          detail: "",
          remediation: "",
        }],
      },
    }), onOpenWeb);

    const rail = screen.getByRole("complementary", { name: "需要关注的项目" });
    // A file the service prepares on start is not an environment problem.
    expect(within(rail).queryByText("环境问题")).not.toBeInTheDocument();
    expect(within(rail).getByText("图片渲染 Chromium")).toBeInTheDocument();
    expect(screen.queryByText("platform.resource_missing")).not.toBeInTheDocument();
    fireEvent.click(within(rail).getByRole("button", { name: "在管理界面准备" }));
    expect(onOpenWeb).toHaveBeenCalledOnce();
  });

  test("separates settings reading mode from its editable controls", () => {
    const props = {
      snapshot: configuredSnapshot,
      settingsDraft: configuredSnapshot.launcher.settings,
      resolvedSettings: configuredSnapshot.launcher.resolvedSettings,
      busyAction: null,
      controlsDisabled: false,
      onUpdateInstallationRoot: noop,
      onUpdateCloseBehavior: noop,
      onUpdateAdvancedOverride: noop,
      onChooseInstallationRoot: noop,
      onChooseServer: noop,
      onChooseConfig: noop,
      onChooseWorkdir: noop,
      onResetAdmin: noop,
      onExit: noop,
    };
    const { rerender } = render(<AppShellSettingsSection {...props} editingSettings={false} />);

    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(screen.getByText("每次询问")).toBeInTheDocument();

    rerender(<AppShellSettingsSection {...props} editingSettings />);

    expect(screen.getByRole("textbox", { name: "安装目录" })).toHaveValue("C:\\RayleaBot");
    expect(screen.getByRole("radio", { name: /每次询问/ })).toBeChecked();
  });

  test("keeps technical diagnostics collapsed and promotes real stderr", () => {
    const { rerender } = render(
      <AppShellDiagnosticsSection
        snapshot={configuredSnapshot}
        diagnosticsSummary="状态摘要：未启动"
        onOpenLogs={noop}
      />,
    );

    const disclosure = screen.getByText("技术详情").closest("details");
    expect(disclosure).not.toHaveAttribute("open");
    expect(screen.getByText("当前没有新的异常输出")).toBeInTheDocument();

    rerender(
      <AppShellDiagnosticsSection
        snapshot={createLauncherSnapshot({ launcher: { recentStderr: ["listen failed"] } })}
        diagnosticsSummary="状态摘要：启动失败"
        onOpenLogs={noop}
      />,
    );

    expect(screen.getByText("listen failed")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "异常输出" })).toBeInTheDocument();
  });

  test("does not report a clean log for a service whose output the Launcher never captured", () => {
    const snapshot = createLauncherSnapshot({
      server: { health: { status: "ok" }, readiness: { status: "ready" } },
      launcher: { processOwnership: "external" },
    });
    render(<AppShellDiagnosticsSection snapshot={snapshot} diagnosticsSummary="" onOpenLogs={noop} />);

    expect(screen.getByText("未捕获服务输出", { selector: "dd" })).toBeInTheDocument();
    expect(screen.getByText("启动器未捕获该服务的输出，请在管理界面的实时日志查看。")).toBeInTheDocument();
    expect(screen.queryByText(/未发现异常输出|当前没有新的异常输出/)).not.toBeInTheDocument();
  });

  // The HarmonyOS Sans agreement requires a notice in the software that the fonts are used.
  test("states the bundled HarmonyOS Sans interface font", () => {
    render(
      <AppShellAboutSection
        snapshot={configuredSnapshot}
        controlsDisabled={false}
        onApplyUpdate={noop}
        onCheckForUpdates={noop}
        onOpenReleasePage={noop}
        onOpenRepositoryPage={noop}
      />,
    );

    expect(screen.getByText("界面字体")).toBeInTheDocument();
    expect(screen.getByText("HarmonyOS Sans SC")).toBeInTheDocument();
  });

  test("explains unavailable updates without rendering a broken action", () => {
    render(
      <AppShellAboutSection
        snapshot={configuredSnapshot}
        controlsDisabled={false}
        onApplyUpdate={noop}
        onCheckForUpdates={noop}
        onOpenReleasePage={noop}
        onOpenRepositoryPage={noop}
      />,
    );

    expect(screen.getByText("当前构建不提供更新检查")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "检查更新" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "GitHub" })).toBeEnabled();
  });

  test("shows the version as loading while the first update check runs", () => {
    render(
      <AppShellAboutSection
        snapshot={createLauncherSnapshot({ launcher: { releaseCheck: { status: "checking", currentVersion: "" } } })}
        controlsDisabled={false}
        onApplyUpdate={noop}
        onCheckForUpdates={noop}
        onOpenReleasePage={noop}
        onOpenRepositoryPage={noop}
      />,
    );

    expect(screen.getByText("读取中")).toBeInTheDocument();
    expect(screen.queryByText("开发")).not.toBeInTheDocument();
  });

  test("offers one-click update and the release page when a newer version exists", () => {
    const onApplyUpdate = vi.fn();
    const onOpenReleasePage = vi.fn();
    const snapshot = createLauncherSnapshot({
      launcher: {
        releaseCheck: {
          status: "update_available",
          currentVersion: "0.3.0",
          latestVersion: "0.4.0",
          releasePageUrl: "https://example.invalid/releases/v0.4.0",
          updateAvailable: true,
          canCheck: true,
        },
      },
    });

    render(
      <AppShellAboutSection
        snapshot={snapshot}
        controlsDisabled={false}
        onApplyUpdate={onApplyUpdate}
        onCheckForUpdates={noop}
        onOpenReleasePage={onOpenReleasePage}
        onOpenRepositoryPage={noop}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "立即更新" }));
    fireEvent.click(screen.getByRole("button", { name: "发布页" }));
    expect(onApplyUpdate).toHaveBeenCalledOnce();
    expect(onOpenReleasePage).toHaveBeenCalledOnce();
  });

  test("keeps the update action disabled while an update runs", () => {
    const snapshot = createLauncherSnapshot({
      launcher: {
        releaseCheck: {
          status: "updating",
          currentVersion: "0.3.0",
          latestVersion: "0.4.0",
          summary: "正在下载更新。",
          updateAvailable: true,
          canCheck: false,
        },
      },
    });

    render(
      <AppShellAboutSection
        snapshot={snapshot}
        controlsDisabled={false}
        onApplyUpdate={noop}
        onCheckForUpdates={noop}
        onOpenReleasePage={noop}
        onOpenRepositoryPage={noop}
      />,
    );

    expect(screen.getByRole("button", { name: "正在更新" })).toBeDisabled();
    expect(screen.getByText("正在下载更新。")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "立即更新" })).not.toBeInTheDocument();
  });

  test("offers the release page for platform-guided builds", () => {
    const onOpenReleasePage = vi.fn();
    const snapshot = createLauncherSnapshot({
      launcher: {
        releaseCheck: {
          status: "disabled",
          currentVersion: "0.3.0",
          releasePageUrl: "https://example.invalid/releases/latest",
          updateAvailable: false,
          canCheck: false,
        },
      },
    });

    render(
      <AppShellAboutSection
        snapshot={snapshot}
        controlsDisabled={false}
        onApplyUpdate={noop}
        onCheckForUpdates={noop}
        onOpenReleasePage={onOpenReleasePage}
        onOpenRepositoryPage={noop}
      />,
    );

    expect(screen.getByText("0.3.0")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "打开发布页" }));
    expect(onOpenReleasePage).toHaveBeenCalledOnce();
  });

  test("shows the specific update error code and complete reason", () => {
    const snapshot = createLauncherSnapshot({
      launcher: {
        releaseCheck: {
          status: "failed",
          currentVersion: "0.3.0",
          summary: "检查更新失败",
          detail: "无法获取发布信息，请稍后重试。",
          errorCode: "launcher.update_check_failed",
          canCheck: true,
        },
      },
    });

    render(
      <AppShellAboutSection
        snapshot={snapshot}
        controlsDisabled={false}
        onApplyUpdate={noop}
        onCheckForUpdates={noop}
        onOpenReleasePage={noop}
        onOpenRepositoryPage={noop}
      />,
    );

    // The version line names the state; only the error bar carries the failure text.
    expect(screen.getByText("检查失败")).toBeInTheDocument();
    expect(screen.getByText("检查更新失败")).toBeInTheDocument();
    expect(screen.getByText("launcher.update_check_failed")).toBeInTheDocument();
    expect(screen.getByText("无法获取发布信息，请稍后重试。")).toBeInTheDocument();
    expect(screen.queryByText("更新请求失败。")).not.toBeInTheDocument();
  });
});

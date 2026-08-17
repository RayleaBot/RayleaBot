// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";

import { AppShellDiagnosticsSection } from "@renderer/AppShellDiagnosticsSection";
import { AppShellEnvironmentSection } from "@renderer/AppShellEnvironmentSection";
import { AppShellAboutSection } from "@renderer/AppShellAboutSection";
import { AppShellSettingsSection } from "@renderer/AppShellSettingsSection";
import { createLauncherSnapshot } from "../helpers/snapshot";

const noop = vi.fn();

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
            title: "服务端可执行文件",
            severity: "ok",
            summary: "已找到可执行文件。",
            detail: "服务端可执行文件可用。",
            remediation: "",
          },
        ],
      },
    });

    render(<AppShellEnvironmentSection snapshot={snapshot} platformLabel="win32-x64" />);

    expect(screen.getByText("配置文件不可读。")).toBeVisible();
    expect(screen.getByText("重新选择有效的配置文件。")).toBeVisible();
    const disclosure = screen.getByText("查看正常项").closest("details");
    expect(disclosure).not.toHaveAttribute("open");
    expect(within(disclosure as HTMLElement).getByText("服务端可执行文件")).toBeInTheDocument();

    fireEvent.click(within(disclosure as HTMLElement).getByText("查看正常项"));
    expect(disclosure).toHaveAttribute("open");
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
    expect(screen.getByText("当前没有新的异常日志")).toBeInTheDocument();

    rerender(
      <AppShellDiagnosticsSection
        snapshot={createLauncherSnapshot({ launcher: { recentStderr: ["listen failed"] } })}
        diagnosticsSummary="状态摘要：启动失败"
        onOpenLogs={noop}
      />,
    );

    expect(screen.getByText("listen failed")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "最近异常输出" })).toBeInTheDocument();
  });

  test("explains unavailable updates without rendering a broken action", () => {
    render(
      <AppShellAboutSection
        snapshot={configuredSnapshot}
        controlsDisabled={false}
        onCheckForUpdates={noop}
        onDownloadUpdate={noop}
        onInstallDownloadedUpdate={noop}
        onOpenReleasePage={noop}
        onOpenRepositoryPage={noop}
      />,
    );

    expect(screen.getByText("当前构建不提供更新检查")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "检查更新" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "GitHub" })).toBeEnabled();
  });

  test("opens the release page for guided updates", () => {
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
          canDownload: false,
          canInstall: false,
        },
      },
    });

    render(
      <AppShellAboutSection
        snapshot={snapshot}
        controlsDisabled={false}
        onCheckForUpdates={noop}
        onDownloadUpdate={noop}
        onInstallDownloadedUpdate={noop}
        onOpenReleasePage={onOpenReleasePage}
        onOpenRepositoryPage={noop}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "打开发布页" }));
    expect(onOpenReleasePage).toHaveBeenCalledOnce();
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
          canDownload: false,
          canInstall: false,
        },
      },
    });

    render(
      <AppShellAboutSection
        snapshot={snapshot}
        controlsDisabled={false}
        onCheckForUpdates={noop}
        onDownloadUpdate={noop}
        onInstallDownloadedUpdate={noop}
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
          summary: "发布签名验证失败",
          detail: "没有受信任的 Ed25519 公钥接受当前发布清单签名。",
          errorCode: "release.signature_invalid",
          canCheck: true,
        },
      },
    });

    render(
      <AppShellAboutSection
        snapshot={snapshot}
        controlsDisabled={false}
        onCheckForUpdates={noop}
        onDownloadUpdate={noop}
        onInstallDownloadedUpdate={noop}
        onOpenReleasePage={noop}
        onOpenRepositoryPage={noop}
      />,
    );

    expect(screen.getAllByText("发布签名验证失败")).toHaveLength(2);
    expect(screen.getByText("release.signature_invalid")).toBeInTheDocument();
    expect(screen.getByText("没有受信任的 Ed25519 公钥接受当前发布清单签名。")).toBeInTheDocument();
    expect(screen.queryByText("无法确认受信任的更新。")).not.toBeInTheDocument();
  });
});

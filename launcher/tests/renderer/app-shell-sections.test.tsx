// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";

import { AppShellDiagnosticsSection } from "@renderer/AppShellDiagnosticsSection";
import { AppShellEnvironmentSection } from "@renderer/AppShellEnvironmentSection";
import { AppShellAboutSection } from "@renderer/AppShellAboutSection";
import { serviceStateConfig } from "@renderer/AppShell.shared";
import { AppShellSettingsSection } from "@renderer/AppShellSettingsSection";
import { AppShellStatusSection } from "@renderer/AppShellStatusSection";
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
  test("maps service states to their single visual tone vocabulary", () => {
    expect(serviceStateConfig.stopped.tone).toBe("neutral");
    expect(serviceStateConfig.starting.tone).toBe("info");
    expect(serviceStateConfig.running.tone).toBe("success");
    expect(serviceStateConfig.degraded.tone).toBe("warning");
    expect(serviceStateConfig.setup_required.tone).toBe("attention");
    expect(serviceStateConfig.failed.tone).toBe("danger");
  });

  test("keeps a normal stopped state compact and free of attention panels", () => {
    render(
      <AppShellStatusSection
        snapshot={configuredSnapshot}
        resolvedSettings={configuredSnapshot.launcher.resolvedSettings}
        busyAction={null}
        controlsDisabled={false}
        onStart={noop}
        onStop={noop}
        onOpenWeb={noop}
        onOpenRecoveryTasks={noop}
        onOpenRuntimeTasks={noop}
        onOpenLogs={noop}
      />,
    );

    expect(screen.queryByText("运行说明")).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "恢复与准备" })).not.toBeInTheDocument();
    expect(screen.queryByText("服务操作已就绪")).not.toBeInTheDocument();
    expect(screen.getByText("当前没有新的异常日志。")).toBeInTheDocument();
    expect(screen.getByText("服务启动后可进入管理界面")).toBeInTheDocument();
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
        onOpenRepositoryPage={noop}
      />,
    );

    expect(screen.getByText("当前构建不提供更新检查")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "检查更新" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "GitHub" })).toBeEnabled();
  });
});

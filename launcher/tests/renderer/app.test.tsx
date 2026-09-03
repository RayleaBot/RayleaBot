// @vitest-environment jsdom
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, test, vi } from "vitest";
import { App } from "@renderer/App";
import { buildDiagnosticsSummary } from "@renderer/AppState.shared";
import { createLauncherSnapshot } from "../helpers/snapshot";
import type { LauncherDesktopApi } from "@shared/desktop-api";
import type { LauncherSnapshot } from "@shared/launcher-models";

const blankSnapshot: LauncherSnapshot = createLauncherSnapshot();
const TEST_INSTALLATION_ROOT = "C:\\RayleaBotTest\\workspace";

const loadedSnapshot: LauncherSnapshot = createLauncherSnapshot({
  launcher: {
    settings: {
      installationRoot: TEST_INSTALLATION_ROOT,
      closeBehavior: "ask_every_time",
    },
    resolvedSettings: {
      installationRoot: TEST_INSTALLATION_ROOT,
      serverExecutablePath: `${TEST_INSTALLATION_ROOT}\\server\\raylea-server.exe`,
      configPath: `${TEST_INSTALLATION_ROOT}\\config\\user.yaml`,
      workdir: TEST_INSTALLATION_ROOT,
    },
  },
});

const runningManagedSnapshot = createLauncherSnapshot({
  server: {
    health: { status: "ok" },
    readiness: { status: "ready" },
  },
  launcher: {
    ...loadedSnapshot.launcher,
    processId: 4242,
    processLifecycle: "running",
    processOwnership: "launcher_managed",
  },
});

const runningExternalSnapshot = createLauncherSnapshot({
  server: {
    health: { status: "ok" },
    readiness: { status: "ready" },
  },
  launcher: {
    ...loadedSnapshot.launcher,
    processId: null,
    processOwnership: "external",
  },
});

const setupRequiredSnapshot = createLauncherSnapshot({
  server: {
    health: { status: "ok" },
    readiness: {
      status: "setup_required",
      reason: "管理员初始化尚未完成。",
      reason_codes: ["setup.required"],
      checks: { config: "setup_required" },
      issues: [
        {
          code: "setup.required",
          severity: "error",
          summary: "Initial admin setup is required",
          remediation: "请先完成管理员初始化，然后再使用管理入口。",
        },
      ],
    },
  },
  launcher: {
    ...loadedSnapshot.launcher,
    processId: 4242,
    processLifecycle: "running",
    processOwnership: "launcher_managed",
  },
});

function installDesktopApi(api: LauncherDesktopApi) {
  Object.defineProperty(window, "rayleaLauncher", {
    configurable: true,
    value: api,
  });
}

function previewSettings(settings: LauncherSnapshot["launcher"]["settings"]) {
  return {
    installationRoot: settings.installationRoot,
    serverExecutablePath: settings.installationRoot ? `${settings.installationRoot}\\server\\raylea-server.exe` : "",
    configPath: settings.installationRoot ? `${settings.installationRoot}\\config\\user.yaml` : "",
    workdir: settings.installationRoot,
  };
}

afterEach(() => {
  Reflect.deleteProperty(window, "rayleaLauncher");
});

describe("App", () => {
  test("hydrates settings from getSnapshot after initialize resolves", async () => {
    let initialized = false;
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => (initialized ? loadedSnapshot : blankSnapshot)),
      initialize: vi.fn(async () => {
        initialized = true;
      }),
      refresh: vi.fn(async () => undefined),
      start: vi.fn(async () => undefined),
      stop: vi.fn(async () => undefined),
      openWebUi: vi.fn(async () => undefined),
      openReleasePage: vi.fn(async () => undefined),
      checkForUpdates: vi.fn(async () => undefined),
      downloadUpdate: vi.fn(async () => undefined),
      installDownloadedUpdate: vi.fn(async () => undefined),
      openLogsDirectory: vi.fn(async () => undefined),
      saveSettings: vi.fn(async () => undefined),
      previewResolvedSettings: vi.fn(async (settings) => previewSettings(settings)),
      chooseInstallationRoot: vi.fn(async () => null),
      chooseServerExecutable: vi.fn(async () => null),
      chooseConfigFile: vi.fn(async () => null),
      chooseWorkdir: vi.fn(async () => null),
      exitApplication: vi.fn(async () => undefined),
      minimize: vi.fn(async () => undefined),
      maximize: vi.fn(async () => undefined),
      close: vi.fn(async () => undefined),
      closeConfirmResponse: vi.fn(async () => undefined),
      isMaximized: vi.fn(async () => false),
      onSnapshot: vi.fn(() => () => undefined),
      onMaximizedChange: vi.fn(() => () => undefined),
      onShowExitConfirm: vi.fn(() => () => undefined),
      hasPendingCloseConfirm: vi.fn(async () => false),
      onShowExternalStopConfirm: vi.fn(() => () => undefined),
      hasPendingExternalStopConfirm: vi.fn(async () => false),
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getAllByText(TEST_INSTALLATION_ROOT).length).toBeGreaterThan(0);
    });
  });

  test("restarts the managed service through its secondary restart action", async () => {
    let initialized = false;
    const calls: string[] = [];
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => (initialized ? runningManagedSnapshot : blankSnapshot)),
      initialize: vi.fn(async () => {
        initialized = true;
      }),
      refresh: vi.fn(async () => undefined),
      start: vi.fn(async () => {
        calls.push("start");
      }),
      stop: vi.fn(async () => {
        calls.push("stop");
      }),
      openWebUi: vi.fn(async () => undefined),
      openReleasePage: vi.fn(async () => undefined),
      checkForUpdates: vi.fn(async () => undefined),
      downloadUpdate: vi.fn(async () => undefined),
      installDownloadedUpdate: vi.fn(async () => undefined),
      openLogsDirectory: vi.fn(async () => undefined),
      saveSettings: vi.fn(async () => undefined),
      previewResolvedSettings: vi.fn(async (settings) => previewSettings(settings)),
      chooseInstallationRoot: vi.fn(async () => null),
      chooseServerExecutable: vi.fn(async () => null),
      chooseConfigFile: vi.fn(async () => null),
      chooseWorkdir: vi.fn(async () => null),
      exitApplication: vi.fn(async () => undefined),
      minimize: vi.fn(async () => undefined),
      maximize: vi.fn(async () => undefined),
      close: vi.fn(async () => undefined),
      closeConfirmResponse: vi.fn(async () => undefined),
      isMaximized: vi.fn(async () => false),
      onSnapshot: vi.fn(() => () => undefined),
      onMaximizedChange: vi.fn(() => () => undefined),
      onShowExitConfirm: vi.fn(() => () => undefined),
      hasPendingCloseConfirm: vi.fn(async () => false),
      onShowExternalStopConfirm: vi.fn(() => () => undefined),
      hasPendingExternalStopConfirm: vi.fn(async () => false),
    });

    render(<App />);

    const restartButton = await screen.findByRole("button", { name: "重启服务" });
    fireEvent.click(restartButton);

    await waitFor(() => {
      expect(calls).toEqual(["stop", "start"]);
    });
  });

  test("opens management for an external service without offering start or restart", async () => {
    let initialized = false;
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => (initialized ? runningExternalSnapshot : blankSnapshot)),
      initialize: vi.fn(async () => {
        initialized = true;
      }),
      refresh: vi.fn(async () => undefined),
      start: vi.fn(async () => undefined),
      stop: vi.fn(async () => undefined),
      resetAdmin: vi.fn(async () => undefined),
      openWebUi: vi.fn(async () => undefined),
      openReleasePage: vi.fn(async () => undefined),
      checkForUpdates: vi.fn(async () => undefined),
      downloadUpdate: vi.fn(async () => undefined),
      installDownloadedUpdate: vi.fn(async () => undefined),
      openLogsDirectory: vi.fn(async () => undefined),
      saveSettings: vi.fn(async () => undefined),
      previewResolvedSettings: vi.fn(async (settings) => previewSettings(settings)),
      chooseInstallationRoot: vi.fn(async () => null),
      chooseServerExecutable: vi.fn(async () => null),
      chooseConfigFile: vi.fn(async () => null),
      chooseWorkdir: vi.fn(async () => null),
      exitApplication: vi.fn(async () => undefined),
      minimize: vi.fn(async () => undefined),
      maximize: vi.fn(async () => undefined),
      close: vi.fn(async () => undefined),
      closeConfirmResponse: vi.fn(async () => undefined),
      isMaximized: vi.fn(async () => false),
      onSnapshot: vi.fn(() => () => undefined),
      onMaximizedChange: vi.fn(() => () => undefined),
      onShowExitConfirm: vi.fn(() => () => undefined),
      hasPendingCloseConfirm: vi.fn(async () => false),
      onShowExternalStopConfirm: vi.fn(() => () => undefined),
      hasPendingExternalStopConfirm: vi.fn(async () => false),
    } as LauncherDesktopApi);

    render(<App />);

    const managementButton = await screen.findByRole("button", { name: "管理界面" });
    expect(managementButton).not.toBeDisabled();
    expect(screen.queryByRole("button", { name: "检测到现有服务" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "启动 RayleaBot" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重启服务" })).not.toBeInTheDocument();
  });

  test("keeps first-run setup inside the normal running flow", async () => {
    let initialized = false;
    const openWebUi = vi.fn(async () => undefined);
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => (initialized ? setupRequiredSnapshot : blankSnapshot)),
      initialize: vi.fn(async () => {
        initialized = true;
      }),
      refresh: vi.fn(async () => undefined),
      start: vi.fn(async () => undefined),
      stop: vi.fn(async () => undefined),
      resetAdmin: vi.fn(async () => undefined),
      openWebUi,
      openReleasePage: vi.fn(async () => undefined),
      checkForUpdates: vi.fn(async () => undefined),
      downloadUpdate: vi.fn(async () => undefined),
      installDownloadedUpdate: vi.fn(async () => undefined),
      openLogsDirectory: vi.fn(async () => undefined),
      saveSettings: vi.fn(async () => undefined),
      previewResolvedSettings: vi.fn(async (settings) => previewSettings(settings)),
      chooseInstallationRoot: vi.fn(async () => null),
      chooseServerExecutable: vi.fn(async () => null),
      chooseConfigFile: vi.fn(async () => null),
      chooseWorkdir: vi.fn(async () => null),
      exitApplication: vi.fn(async () => undefined),
      minimize: vi.fn(async () => undefined),
      maximize: vi.fn(async () => undefined),
      close: vi.fn(async () => undefined),
      closeConfirmResponse: vi.fn(async () => undefined),
      isMaximized: vi.fn(async () => false),
      onSnapshot: vi.fn(() => () => undefined),
      onMaximizedChange: vi.fn(() => () => undefined),
      onShowExitConfirm: vi.fn(() => () => undefined),
      hasPendingCloseConfirm: vi.fn(async () => false),
      onShowExternalStopConfirm: vi.fn(() => () => undefined),
      hasPendingExternalStopConfirm: vi.fn(async () => false),
    } as LauncherDesktopApi);

    render(<App />);

    const managementButton = await screen.findByRole("button", { name: "管理界面" });
    expect(managementButton).not.toBeDisabled();
    fireEvent.click(document.querySelector<HTMLButtonElement>(".service-control__primary")!);
    await waitFor(() => expect(openWebUi).toHaveBeenCalledOnce());
    expect(screen.getAllByText("运行中").length).toBeGreaterThan(0);
    expect(screen.queryByText("管理员初始化尚未完成。")).not.toBeInTheDocument();
    expect(screen.queryByText("需要设置")).not.toBeInTheDocument();
    expect(screen.queryByText("需要处理")).not.toBeInTheDocument();
    expect(screen.queryByText("服务诊断")).not.toBeInTheDocument();
    expect(screen.queryByText("setup.required")).not.toBeInTheDocument();
    expect(screen.queryByText("请先完成管理员初始化，然后再使用管理入口。")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "打开初始化" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "执行恢复检查" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "准备运行环境" })).not.toBeInTheDocument();

    const diagnosticsSummary = buildDiagnosticsSummary(setupRequiredSnapshot);
    expect(diagnosticsSummary).not.toContain("管理员初始化");
    expect(diagnosticsSummary).not.toContain("setup.required");
    expect(diagnosticsSummary).not.toContain("setup_required");
  });

  test("previews derived settings while editing the installation root", async () => {
    let initialized = false;
    const previewResolvedSettings = vi.fn(async (settings: LauncherSnapshot["launcher"]["settings"]) => ({
      installationRoot: settings.installationRoot,
      serverExecutablePath: `${settings.installationRoot}\\server\\raylea-server.exe`,
      configPath: `${settings.installationRoot}\\config\\user.yaml`,
      workdir: settings.installationRoot,
    }));

    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => (initialized ? loadedSnapshot : blankSnapshot)),
      initialize: vi.fn(async () => {
        initialized = true;
      }),
      refresh: vi.fn(async () => undefined),
      start: vi.fn(async () => undefined),
      stop: vi.fn(async () => undefined),
      openWebUi: vi.fn(async () => undefined),
      openReleasePage: vi.fn(async () => undefined),
      checkForUpdates: vi.fn(async () => undefined),
      downloadUpdate: vi.fn(async () => undefined),
      installDownloadedUpdate: vi.fn(async () => undefined),
      openLogsDirectory: vi.fn(async () => undefined),
      saveSettings: vi.fn(async () => undefined),
      previewResolvedSettings: vi.fn(async (settings) => previewSettings(settings)),
      chooseInstallationRoot: vi.fn(async () => null),
      chooseServerExecutable: vi.fn(async () => null),
      chooseConfigFile: vi.fn(async () => null),
      chooseWorkdir: vi.fn(async () => null),
      exitApplication: vi.fn(async () => undefined),
      minimize: vi.fn(async () => undefined),
      maximize: vi.fn(async () => undefined),
      close: vi.fn(async () => undefined),
      isMaximized: vi.fn(async () => false),
      onSnapshot: vi.fn(() => () => undefined),
      onMaximizedChange: vi.fn(() => () => undefined),
      onShowExitConfirm: vi.fn(() => () => undefined),
      hasPendingCloseConfirm: vi.fn(async () => false),
      onShowExternalStopConfirm: vi.fn(() => () => undefined),
      hasPendingExternalStopConfirm: vi.fn(async () => false),
      previewResolvedSettings,
    } as LauncherDesktopApi);

    render(<App />);

    await waitFor(() => {
      expect(screen.getAllByText(TEST_INSTALLATION_ROOT).length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getByRole("button", { name: "偏好设置" }));
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "偏好设置" })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: "编辑配置" }));

    const installInput = screen.getByRole("textbox", { name: "安装目录" });
    fireEvent.change(installInput, { target: { value: "D:\\RayleaPortable" } });

    await waitFor(() => {
      expect(previewResolvedSettings).toHaveBeenCalled();
      expect(screen.getByRole("textbox", { name: "服务端程序" })).toHaveValue("D:\\RayleaPortable\\server\\raylea-server.exe");
      expect(screen.getByRole("textbox", { name: "配置文件" })).toHaveValue("D:\\RayleaPortable\\config\\user.yaml");
    });
  });

  test("restores a pending close confirmation until cancellation succeeds", async () => {
    let initialized = false;
    let rejectCloseConfirmation: (reason: Error) => void = () => undefined;
    const closeConfirmResponse = vi.fn()
      .mockImplementationOnce(() => new Promise<void>((_resolve, reject) => {
        rejectCloseConfirmation = reject;
      }))
      .mockResolvedValue(undefined);
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => (initialized ? loadedSnapshot : blankSnapshot)),
      initialize: vi.fn(async () => { initialized = true; }),
      refresh: vi.fn(async () => undefined),
      start: vi.fn(async () => undefined),
      stop: vi.fn(async () => undefined),
      resetAdmin: vi.fn(async () => undefined),
      checkForUpdates: vi.fn(async () => undefined),
      downloadUpdate: vi.fn(async () => undefined),
      installDownloadedUpdate: vi.fn(async () => undefined),
      openWebUi: vi.fn(async () => undefined),
      openReleasePage: vi.fn(async () => undefined),
      openRepositoryPage: vi.fn(async () => undefined),
      openLogsDirectory: vi.fn(async () => undefined),
      saveSettings: vi.fn(async () => undefined),
      previewResolvedSettings: vi.fn(async (settings) => previewSettings(settings)),
      chooseInstallationRoot: vi.fn(async () => null),
      chooseServerExecutable: vi.fn(async () => null),
      chooseConfigFile: vi.fn(async () => null),
      chooseWorkdir: vi.fn(async () => null),
      exitApplication: vi.fn(async () => undefined),
      minimize: vi.fn(async () => undefined),
      maximize: vi.fn(async () => undefined),
      close: vi.fn(async () => undefined),
      closeConfirmResponse,
      setThemeMode: vi.fn(async () => undefined),
      isMaximized: vi.fn(async () => false),
      onSnapshot: vi.fn(() => () => undefined),
      onMaximizedChange: vi.fn(() => () => undefined),
      onShowExitConfirm: vi.fn(() => () => undefined),
      hasPendingCloseConfirm: vi.fn(async () => true),
      onShowExternalStopConfirm: vi.fn(() => () => undefined),
      hasPendingExternalStopConfirm: vi.fn(async () => false),
    } as LauncherDesktopApi);

    render(<App />);
    await screen.findByText(TEST_INSTALLATION_ROOT);
    fireEvent.click(await screen.findByRole("button", { name: "取消" }));

    await waitFor(() => {
      expect(closeConfirmResponse).toHaveBeenCalledWith({ action: "cancel", setAsDefault: false });
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    await act(async () => {
      rejectCloseConfirmation(new Error("desktop bridge unavailable"));
    });

    fireEvent.click(await screen.findByRole("button", { name: "取消" }));
    await waitFor(() => {
      expect(closeConfirmResponse).toHaveBeenCalledTimes(2);
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });

  test("always resolves the external-service stop confirmation", async () => {
    let initialized = false;
    let showExternalStopConfirm: (() => void) | undefined;
    const externalStopConfirmResponse = vi.fn(async () => undefined);
    installDesktopApi({
      getPlatform: vi.fn(async () => "linux-x64"),
      getSnapshot: vi.fn(async () => (initialized ? loadedSnapshot : blankSnapshot)),
      initialize: vi.fn(async () => { initialized = true; }),
      refresh: vi.fn(async () => undefined),
      start: vi.fn(async () => undefined),
      stop: vi.fn(async () => undefined),
      resetAdmin: vi.fn(async () => undefined),
      checkForUpdates: vi.fn(async () => undefined),
      downloadUpdate: vi.fn(async () => undefined),
      installDownloadedUpdate: vi.fn(async () => undefined),
      openWebUi: vi.fn(async () => undefined),
      openReleasePage: vi.fn(async () => undefined),
      openRepositoryPage: vi.fn(async () => undefined),
      openLogsDirectory: vi.fn(async () => undefined),
      saveSettings: vi.fn(async () => undefined),
      previewResolvedSettings: vi.fn(async (settings) => previewSettings(settings)),
      chooseInstallationRoot: vi.fn(async () => null),
      chooseServerExecutable: vi.fn(async () => null),
      chooseConfigFile: vi.fn(async () => null),
      chooseWorkdir: vi.fn(async () => null),
      exitApplication: vi.fn(async () => undefined),
      minimize: vi.fn(async () => undefined),
      maximize: vi.fn(async () => undefined),
      close: vi.fn(async () => undefined),
      closeConfirmResponse: vi.fn(async () => undefined),
      hasPendingCloseConfirm: vi.fn(async () => false),
      externalStopConfirmResponse,
      hasPendingExternalStopConfirm: vi.fn(async () => true),
      setThemeMode: vi.fn(async () => undefined),
      isMaximized: vi.fn(async () => false),
      onSnapshot: vi.fn(() => () => undefined),
      onMaximizedChange: vi.fn(() => () => undefined),
      onShowExitConfirm: vi.fn(() => () => undefined),
      onShowExternalStopConfirm: vi.fn((listener) => {
        showExternalStopConfirm = listener;
        return () => undefined;
      }),
    });

    render(<App />);
    await screen.findByText(TEST_INSTALLATION_ROOT);

    const externalStopDialog = await screen.findByRole("dialog", { name: "停止现有服务" });
    fireEvent.keyDown(externalStopDialog, { key: "Escape", code: "Escape" });
    await waitFor(() => {
      expect(externalStopConfirmResponse).toHaveBeenLastCalledWith(false);
      expect(screen.queryByRole("dialog", { name: "停止现有服务" })).not.toBeInTheDocument();
    });

    act(() => showExternalStopConfirm?.());
    const confirmedDialog = await screen.findByRole("dialog", { name: "停止现有服务" });
    fireEvent.click(within(confirmedDialog).getByRole("button", { name: "停止服务" }));
    await waitFor(() => {
      expect(externalStopConfirmResponse).toHaveBeenLastCalledWith(true);
    });
  });
});

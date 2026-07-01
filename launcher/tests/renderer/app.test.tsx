// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, test, vi } from "vitest";
import { App } from "@renderer/App";
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
    readiness: { status: "setup_required", reason: "管理员初始化尚未完成。" },
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
      retry: vi.fn(async () => undefined),
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
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getAllByText(TEST_INSTALLATION_ROOT).length).toBeGreaterThan(0);
    });
  });

  test("shows a dedicated loading shell while initialization is still running", async () => {
    let resolveInitialize: (() => void) | null = null;
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => blankSnapshot),
      initialize: vi.fn(
        () =>
          new Promise<void>((resolve) => {
            resolveInitialize = resolve;
          }),
      ),
      refresh: vi.fn(async () => undefined),
      retry: vi.fn(async () => undefined),
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
    });

    render(<App />);

    expect(screen.getByText("正在准备启动器")).toBeInTheDocument();
    expect(screen.queryByText("正在加载启动器设置...")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "启动 RayleaBot" })).not.toBeInTheDocument();

    resolveInitialize?.();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "启动 RayleaBot" })).toBeEnabled();
    });
  });

  test("restarts the managed service when the running-state primary action is clicked", async () => {
    let initialized = false;
    const calls: string[] = [];
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => (initialized ? runningManagedSnapshot : blankSnapshot)),
      initialize: vi.fn(async () => {
        initialized = true;
      }),
      refresh: vi.fn(async () => undefined),
      retry: vi.fn(async () => undefined),
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
    });

    render(<App />);

    const restartButton = await screen.findByRole("button", { name: "重启服务" });
    fireEvent.click(restartButton);

    await waitFor(() => {
      expect(calls).toEqual(["stop", "start"]);
    });
  });

  test("shows a disabled external-service action instead of start or restart", async () => {
    let initialized = false;
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => (initialized ? runningExternalSnapshot : blankSnapshot)),
      initialize: vi.fn(async () => {
        initialized = true;
      }),
      refresh: vi.fn(async () => undefined),
      retry: vi.fn(async () => undefined),
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
    } as LauncherDesktopApi);

    render(<App />);

    const externalButton = await screen.findByRole("button", { name: "检测到现有服务" });
    expect(externalButton).toBeDisabled();
    expect(screen.queryByRole("button", { name: "启动 RayleaBot" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重启服务" })).not.toBeInTheDocument();
  });

  test("disables recovery actions while setup is still required", async () => {
    let initialized = false;
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => (initialized ? setupRequiredSnapshot : blankSnapshot)),
      initialize: vi.fn(async () => {
        initialized = true;
      }),
      refresh: vi.fn(async () => undefined),
      retry: vi.fn(async () => undefined),
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
    } as LauncherDesktopApi);

    render(<App />);

    expect(await screen.findByRole("button", { name: "执行恢复检查" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "准备运行环境" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "管理界面" })).not.toBeDisabled();
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
      retry: vi.fn(async () => undefined),
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
});

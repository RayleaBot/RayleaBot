// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, test, vi } from "vitest";
import { App } from "@renderer/App";
import type { LauncherDesktopApi } from "@shared/desktop-api";
import type { LauncherSnapshot } from "@shared/launcher-models";

const blankSnapshot: LauncherSnapshot = {
  settings: {
    installationRoot: "",
    closeBehavior: "ask_every_time",
  },
  resolvedSettings: {
    installationRoot: "",
    serverExecutablePath: "",
    configPath: "",
    workdir: "",
  },
  endpoint: {
    host: "127.0.0.1",
    port: 8080,
    baseUrl: "http://127.0.0.1:8080/",
  },
  environmentChecks: [],
  recentStderr: [],
  processId: null,
  serviceState: "stopped",
  serviceOwnership: "none",
  shutdownRequested: false,
  serviceDetail: "服务尚未启动。",
  lastError: "",
  releaseCheck: {
    status: "unavailable",
    currentVersion: "",
    latestVersion: "",
    summary: "版本信息不可用",
    detail: "",
    releasePageUrl: "",
    updateAvailable: false,
  },
};

const loadedSnapshot: LauncherSnapshot = {
  ...blankSnapshot,
  settings: {
    installationRoot: "C:\\Users\\26789\\Desktop\\RayleaBot",
    closeBehavior: "ask_every_time",
  },
  resolvedSettings: {
    installationRoot: "C:\\Users\\26789\\Desktop\\RayleaBot",
    serverExecutablePath: "C:\\Users\\26789\\Desktop\\RayleaBot\\server\\raylea-server.exe",
    configPath: "C:\\Users\\26789\\Desktop\\RayleaBot\\config\\user.yaml",
    workdir: "C:\\Users\\26789\\Desktop\\RayleaBot",
  },
  serviceDetail: "服务尚未启动。",
};

const runningManagedSnapshot = Object.assign({}, loadedSnapshot, {
  processId: 4242,
  serviceState: "running",
  serviceOwnership: "launcher_managed",
  serviceDetail: "服务正在运行。",
}) as LauncherSnapshot;

const runningExternalSnapshot = Object.assign({}, loadedSnapshot, {
  processId: null,
  serviceState: "running",
  serviceOwnership: "external",
  serviceDetail: "检测到现有服务。",
}) as LauncherSnapshot;

const setupRequiredSnapshot = Object.assign({}, loadedSnapshot, {
  processId: 4242,
  serviceState: "setup_required",
  serviceOwnership: "launcher_managed",
  serviceDetail: "管理员初始化尚未完成。",
}) as LauncherSnapshot;

function installDesktopApi(api: LauncherDesktopApi) {
  Object.defineProperty(window, "rayleaLauncher", {
    configurable: true,
    value: api,
  });
}

function previewSettings(settings: LauncherSnapshot["settings"]) {
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
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getAllByText("C:\\Users\\26789\\Desktop\\RayleaBot").length).toBeGreaterThan(0);
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
      createRecoveryRecheck: vi.fn(async () => undefined),
      createRuntimeBootstrap: vi.fn(async () => undefined),
      openReleasePage: vi.fn(async () => undefined),
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
      createRecoveryRecheck: vi.fn(async () => undefined),
      createRuntimeBootstrap: vi.fn(async () => undefined),
      openReleasePage: vi.fn(async () => undefined),
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
    } as LauncherDesktopApi);

    render(<App />);

    expect(await screen.findByRole("button", { name: "重新检查" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "准备运行时" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "管理面板" })).not.toBeDisabled();
  });

  test("previews derived settings while editing the installation root", async () => {
    let initialized = false;
    const previewResolvedSettings = vi.fn(async (settings: LauncherSnapshot["settings"]) => ({
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
      previewResolvedSettings,
    } as LauncherDesktopApi);

    render(<App />);

    await waitFor(() => {
      expect(screen.getAllByText("C:\\Users\\26789\\Desktop\\RayleaBot").length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getByRole("button", { name: "偏好设置" }));
    fireEvent.click(screen.getByRole("button", { name: "编辑配置" }));

    const installInput = screen.getByRole("textbox", { name: "安装目录" });
    fireEvent.change(installInput, { target: { value: "D:\\RayleaPortable" } });

    await waitFor(() => {
      expect(previewResolvedSettings).toHaveBeenCalled();
      expect(screen.getByText("D:\\RayleaPortable\\server\\raylea-server.exe")).toBeInTheDocument();
      expect(screen.getByText("D:\\RayleaPortable\\config\\user.yaml")).toBeInTheDocument();
    });
  });
});

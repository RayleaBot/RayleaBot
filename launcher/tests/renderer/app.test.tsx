// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, test, vi } from "vitest";
import { App } from "@renderer/App";
import type { LauncherDesktopApi } from "@shared/desktop-api";
import type { LauncherSnapshot } from "@shared/launcher-models";

const blankSnapshot: LauncherSnapshot = {
  settings: {
    serverExecutablePath: "",
    configPath: "",
    workdir: "",
    closeBehavior: "ask_every_time",
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
    serverExecutablePath: "C:\\Users\\26789\\Desktop\\RayleaBot\\server\\raylea-server.exe",
    configPath: "C:\\Users\\26789\\Desktop\\RayleaBot\\config\\user.yaml",
    workdir: "C:\\Users\\26789\\Desktop\\RayleaBot",
    closeBehavior: "ask_every_time",
  },
  serviceDetail: "服务尚未启动。",
};

const readySnapshot: LauncherSnapshot = {
  ...loadedSnapshot,
  processId: 4242,
  serviceState: "ready",
  serviceDetail: "服务正在运行。",
};

function installDesktopApi(api: LauncherDesktopApi) {
  Object.defineProperty(window, "rayleaLauncher", {
    configurable: true,
    value: api,
  });
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
      expect(screen.getByText(/C:\\Users\\26789\\Desktop\\RayleaBot/)).toBeInTheDocument();
    });
  });

  test("keeps service actions disabled while initialization is still running", async () => {
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

    // While initializing, the start button should be disabled
    const startButton = screen.getByRole("button", { name: "启动 RayleaBot" });
    expect(startButton).toBeDisabled();

    // After initialize resolves, the button should be enabled
    resolveInitialize?.();
    await waitFor(() => {
      expect(startButton).not.toBeDisabled();
    });
  });

  test("restarts the managed service when the ready-state primary action is clicked", async () => {
    let initialized = false;
    const calls: string[] = [];
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => (initialized ? readySnapshot : blankSnapshot)),
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
});

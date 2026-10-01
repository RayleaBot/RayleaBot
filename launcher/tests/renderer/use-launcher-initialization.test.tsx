// @vitest-environment jsdom
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, test, vi } from "vitest";
import type { LauncherDesktopApi } from "@shared/desktop-api";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { createLauncherSnapshot } from "../helpers/snapshot";

import { useLauncherInitialization } from "@renderer/useLauncherInitialization";

const blankSnapshot: LauncherSnapshot = createLauncherSnapshot();

function installDesktopApi(api: LauncherDesktopApi) {
  Object.defineProperty(window, "rayleaLauncher", {
    configurable: true,
    value: api,
  });
}

afterEach(() => {
  Reflect.deleteProperty(window, "rayleaLauncher");
});

describe("useLauncherInitialization", () => {
  test("keeps live snapshot and window updates when initial reads resolve late", async () => {
    let snapshotListener!: (snapshot: LauncherSnapshot) => void;
    let maximizedListener!: (value: boolean) => void;
    let resolveSnapshot!: (snapshot: LauncherSnapshot) => void;
    let resolveMaximized!: (value: boolean) => void;
    const snapshotRequest = new Promise<LauncherSnapshot>((resolve) => { resolveSnapshot = resolve; });
    const maximizedRequest = new Promise<boolean>((resolve) => { resolveMaximized = resolve; });
    const getSnapshot = vi.fn(() => snapshotRequest);
    const liveSnapshot = createLauncherSnapshot({ launcher: { processLifecycle: "running" } });
    installDesktopApi({
      initialize: vi.fn(async () => undefined),
      getSnapshot,
      getPlatform: vi.fn(async () => "win32-x64"),
      isMaximized: vi.fn(() => maximizedRequest),
      onSnapshot: vi.fn((listener) => { snapshotListener = listener; return () => undefined; }),
      onMaximizedChange: vi.fn((listener) => { maximizedListener = listener; return () => undefined; }),
    } as unknown as LauncherDesktopApi);
    const { result } = renderHook(() => useLauncherInitialization());
    await waitFor(() => expect(getSnapshot).toHaveBeenCalledOnce());
    await act(async () => {
      snapshotListener(liveSnapshot);
      maximizedListener(true);
      resolveSnapshot(blankSnapshot);
      resolveMaximized(false);
    });
    expect(result.current.snapshot.launcher.processLifecycle).toBe("running");
    expect(result.current.isMaximized).toBe(true);
    expect(result.current.initializing).toBe(false);
  });

  test("unsubscribes and avoids reading a snapshot after initialization outlives the component", async () => {
    let finishInitialize!: () => void;
    const initializing = new Promise<void>((resolve) => { finishInitialize = resolve; });
    const unsubscribeSnapshot = vi.fn();
    const unsubscribeMaximized = vi.fn();
    const getSnapshot = vi.fn(async () => blankSnapshot);
    installDesktopApi({
      initialize: vi.fn(() => initializing),
      getSnapshot,
      getPlatform: vi.fn(async () => "win32-x64"),
      isMaximized: vi.fn(async () => false),
      onSnapshot: vi.fn(() => unsubscribeSnapshot),
      onMaximizedChange: vi.fn(() => unsubscribeMaximized),
    } as unknown as LauncherDesktopApi);
    const { unmount } = renderHook(() => useLauncherInitialization());
    unmount();
    await act(async () => { finishInitialize(); });
    expect(unsubscribeSnapshot).toHaveBeenCalledOnce();
    expect(unsubscribeMaximized).toHaveBeenCalledOnce();
    expect(getSnapshot).not.toHaveBeenCalled();
  });

  test("subscribes before initialize and retains snapshots emitted during initialization", async () => {
    let snapshotListener: ((snapshot: LauncherSnapshot) => void) | undefined;
    const initializedSnapshot = createLauncherSnapshot({
      launcher: {
        processLifecycle: "running",
        processOwnership: "launcher_managed",
      },
    });
    const onSnapshot = vi.fn((listener: (snapshot: LauncherSnapshot) => void) => {
      snapshotListener = listener;
      return () => undefined;
    });
    const initialize = vi.fn(async () => {
      expect(snapshotListener).toBeDefined();
      snapshotListener?.(initializedSnapshot);
    });

    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => initializedSnapshot),
      initialize,
      refresh: vi.fn(async () => undefined),
      start: vi.fn(async () => undefined),
      stop: vi.fn(async () => undefined),
      openWebUi: vi.fn(async () => undefined),
      openReleasePage: vi.fn(async () => undefined),
      checkForUpdates: vi.fn(async () => undefined),
      applyUpdate: vi.fn(async () => undefined),
      openLogsDirectory: vi.fn(async () => undefined),
      saveSettings: vi.fn(async () => undefined),
      previewResolvedSettings: vi.fn(async () => blankSnapshot.launcher.resolvedSettings),
      chooseInstallationRoot: vi.fn(async () => null),
      chooseServerExecutable: vi.fn(async () => null),
      chooseConfigFile: vi.fn(async () => null),
      chooseWorkdir: vi.fn(async () => null),
      exitApplication: vi.fn(async () => undefined),
      minimize: vi.fn(async () => undefined),
      maximize: vi.fn(async () => undefined),
      close: vi.fn(async () => undefined),
      isMaximized: vi.fn(async () => true),
      onSnapshot,
      onMaximizedChange: vi.fn(() => () => undefined),
    } as LauncherDesktopApi);

    const { result } = renderHook(() => useLauncherInitialization());

    await waitFor(() => {
      expect(result.current.initializing).toBe(false);
      expect(result.current.platformLabel).toBe("win32-x64");
      expect(result.current.isMaximized).toBe(true);
      expect(result.current.snapshot.launcher.processLifecycle).toBe("running");
    });
    expect(onSnapshot.mock.invocationCallOrder[0]).toBeLessThan(initialize.mock.invocationCallOrder[0]);
  });

  test("projects initialization failures into snapshot error state", async () => {
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => blankSnapshot),
      initialize: vi.fn(async () => {
        throw new Error(String.raw`open C:\Users\developer\RayleaBot\config\user.yaml failed`);
      }),
      refresh: vi.fn(async () => undefined),
      start: vi.fn(async () => undefined),
      stop: vi.fn(async () => undefined),
      openWebUi: vi.fn(async () => undefined),
      openReleasePage: vi.fn(async () => undefined),
      checkForUpdates: vi.fn(async () => undefined),
      applyUpdate: vi.fn(async () => undefined),
      openLogsDirectory: vi.fn(async () => undefined),
      saveSettings: vi.fn(async () => undefined),
      previewResolvedSettings: vi.fn(async () => blankSnapshot.launcher.resolvedSettings),
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

    const { result } = renderHook(() => useLauncherInitialization());

    await waitFor(() => {
      expect(result.current.initializing).toBe(false);
      expect(result.current.snapshot.launcher.lastLocalError).toBe("启动器初始化失败。");
      expect(result.current.snapshot.launcher.statusHint).toBe("启动器初始化失败。");
    });
  });

  test("shows the readable message of a desktop boundary error", async () => {
    const message = "启动器设置文件损坏，原文件已保留。请检查 data/launcher.json。";
    installDesktopApi({
      getPlatform: vi.fn(async () => "Windows x64"),
      getSnapshot: vi.fn(async () => blankSnapshot),
      // Wails rejects with the Go error text and carries the marshalled BoundaryError as the cause.
      initialize: vi.fn(async () => {
        throw new Error(`launcher.settings_invalid: ${message}`, { cause: { code: "launcher.settings_invalid", message } });
      }),
      isMaximized: vi.fn(async () => false),
      onSnapshot: vi.fn(() => () => undefined),
      onMaximizedChange: vi.fn(() => () => undefined),
    } as unknown as LauncherDesktopApi);

    const { result } = renderHook(() => useLauncherInitialization());

    await waitFor(() => {
      expect(result.current.initializing).toBe(false);
      expect(result.current.snapshot.launcher.lastLocalError).toBe(message);
    });
  });

  test("handles maximize-state lookup failures without an unhandled rejection", async () => {
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => blankSnapshot),
      initialize: vi.fn(async () => undefined),
      isMaximized: vi.fn(async () => {
        throw new Error("window state unavailable");
      }),
      onSnapshot: vi.fn(() => () => undefined),
      onMaximizedChange: vi.fn(() => () => undefined),
    } as LauncherDesktopApi);

    const { result } = renderHook(() => useLauncherInitialization());

    await waitFor(() => {
      expect(result.current.initializing).toBe(false);
      expect(result.current.isMaximized).toBe(false);
    });
  });
});

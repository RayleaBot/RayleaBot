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
  test("hydrates snapshot, platform, and maximize state after initialize", async () => {
    let snapshotListener: ((snapshot: LauncherSnapshot) => void) | undefined;
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => blankSnapshot),
      initialize: vi.fn(async () => undefined),
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
      onSnapshot: vi.fn((listener) => {
        snapshotListener = listener;
        return () => undefined;
      }),
      onMaximizedChange: vi.fn(() => () => undefined),
    });

    const { result } = renderHook(() => useLauncherInitialization());

    await waitFor(() => {
      expect(result.current.initializing).toBe(false);
      expect(result.current.platformLabel).toBe("win32-x64");
      expect(result.current.isMaximized).toBe(true);
    });

    act(() => {
      snapshotListener?.({
        ...blankSnapshot,
        server: {
          ...blankSnapshot.server,
          health: { status: "ok" },
          readiness: { status: "ready" },
        },
        launcher: {
          ...blankSnapshot.launcher,
          processLifecycle: "running",
          processOwnership: "launcher_managed",
        },
      });
    });

    expect(result.current.snapshot.server.readiness?.status).toBe("ready");
    expect(result.current.snapshot.launcher.processLifecycle).toBe("running");
  });

  test("projects initialization failures into snapshot error state", async () => {
    installDesktopApi({
      getPlatform: vi.fn(async () => "win32-x64"),
      getSnapshot: vi.fn(async () => blankSnapshot),
      initialize: vi.fn(async () => {
        throw new Error("启动器初始化失败");
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
      expect(result.current.snapshot.launcher.lastLocalError).toBe("启动器初始化失败");
      expect(result.current.snapshot.launcher.statusHint).toBe("启动器初始化失败。");
    });
  });
});

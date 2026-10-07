// @vitest-environment jsdom
import { flushPromises } from "@vue/test-utils";
import { afterEach, describe, expect, test, vi } from "vitest";
import { effectScope, ref } from "vue";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { useLauncherConfirmations } from "@renderer/useLauncherConfirmations";
import { useLauncherInitialization } from "@renderer/useLauncherInitialization";
import { useLauncherSettingsState } from "@renderer/useLauncherSettingsState";

import { createDesktopApi, installDesktopApi } from "../helpers/desktop-api";
import { createLauncherSnapshot } from "../helpers/snapshot";

const blankSnapshot: LauncherSnapshot = createLauncherSnapshot();

/** Runs a composable the way a component's setup would; stopping the scope stands in for unmounting. */
function runInScope<T>(composable: () => T) {
  const scope = effectScope();
  const result = scope.run(composable)!;
  return { result, stop: () => scope.stop() };
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
    const getSnapshot = vi.fn(() => new Promise<LauncherSnapshot>((resolve) => { resolveSnapshot = resolve; }));
    installDesktopApi(createDesktopApi({
      getSnapshot,
      isMaximized: vi.fn(() => new Promise<boolean>((resolve) => { resolveMaximized = resolve; })),
      onSnapshot: vi.fn((listener) => { snapshotListener = listener; return () => undefined; }),
      onMaximizedChange: vi.fn((listener) => { maximizedListener = listener; return () => undefined; }),
    }));

    const { result } = runInScope(() => useLauncherInitialization());
    await vi.waitFor(() => expect(getSnapshot).toHaveBeenCalledOnce());
    snapshotListener(createLauncherSnapshot({ launcher: { processLifecycle: "running" } }));
    maximizedListener(true);
    resolveSnapshot(blankSnapshot);
    resolveMaximized(false);
    await flushPromises();

    expect(result.snapshot.value.launcher.processLifecycle).toBe("running");
    expect(result.isMaximized.value).toBe(true);
    expect(result.initializing.value).toBe(false);
  });

  test("unsubscribes and avoids reading a snapshot after initialization outlives the component", async () => {
    let finishInitialize!: () => void;
    const unsubscribeSnapshot = vi.fn();
    const unsubscribeMaximized = vi.fn();
    const getSnapshot = vi.fn(async () => blankSnapshot);
    installDesktopApi(createDesktopApi({
      initialize: vi.fn(() => new Promise<void>((resolve) => { finishInitialize = resolve; })),
      getSnapshot,
      onSnapshot: vi.fn(() => unsubscribeSnapshot),
      onMaximizedChange: vi.fn(() => unsubscribeMaximized),
    }));

    const { stop } = runInScope(() => useLauncherInitialization());
    stop();
    finishInitialize();
    await flushPromises();

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
      snapshotListener?.(initializedSnapshot);
    });
    installDesktopApi(createDesktopApi({
      getSnapshot: vi.fn(async () => initializedSnapshot),
      initialize,
      isMaximized: vi.fn(async () => true),
      onSnapshot,
    }));

    const { result } = runInScope(() => useLauncherInitialization());
    await flushPromises();

    expect(result.initializing.value).toBe(false);
    expect(result.platformLabel.value).toBe("win32-x64");
    expect(result.isMaximized.value).toBe(true);
    expect(result.snapshot.value.launcher.processLifecycle).toBe("running");
    expect(onSnapshot.mock.invocationCallOrder[0]).toBeLessThan(initialize.mock.invocationCallOrder[0]);
  });

  test("projects initialization failures into snapshot error state without exposing the error text", async () => {
    installDesktopApi(createDesktopApi({
      initialize: vi.fn(async () => {
        throw new Error(String.raw`open C:\Users\developer\RayleaBot\config\user.yaml failed`);
      }),
    }));

    const { result } = runInScope(() => useLauncherInitialization());
    await flushPromises();

    expect(result.initializing.value).toBe(false);
    expect(result.snapshot.value.launcher.lastLocalError).toBe("启动器初始化失败。");
    expect(result.snapshot.value.launcher.statusHint).toBe("启动器初始化失败。");
  });

  test("shows the readable message of a desktop boundary error", async () => {
    const message = "启动器设置文件损坏，原文件已保留。请检查 data/launcher.json。";
    installDesktopApi(createDesktopApi({
      // Wails rejects with the Go error text and carries the marshalled BoundaryError as the cause.
      initialize: vi.fn(async () => {
        throw new Error(`launcher.settings_invalid: ${message}`, { cause: { code: "launcher.settings_invalid", message } });
      }),
    }));

    const { result } = runInScope(() => useLauncherInitialization());
    await flushPromises();

    expect(result.snapshot.value.launcher.lastLocalError).toBe(message);
  });

  test("handles maximize-state lookup failures without an unhandled rejection", async () => {
    installDesktopApi(createDesktopApi({
      isMaximized: vi.fn(async () => {
        throw new Error("window state unavailable");
      }),
    }));

    const { result } = runInScope(() => useLauncherInitialization());
    await flushPromises();

    expect(result.initializing.value).toBe(false);
    expect(result.isMaximized.value).toBe(false);
  });
});

describe("useLauncherSettingsState", () => {
  const snapshot = ref(createLauncherSnapshot({
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
  }));

  test("keeps a draft and refreshes preview settings while editing", async () => {
    const previewResolvedSettings = vi.fn(async (settings: LauncherSnapshot["launcher"]["settings"]) => ({
      installationRoot: settings.installationRoot,
      serverExecutablePath: `${settings.installationRoot}\\custom-server.exe`,
      configPath: `${settings.installationRoot}\\custom.yaml`,
      workdir: settings.installationRoot,
    }));
    installDesktopApi(createDesktopApi({ previewResolvedSettings }));

    const { result } = runInScope(() => useLauncherSettingsState(snapshot, ref(true)));
    result.editingDraft.value = { ...snapshot.value.launcher.settings, installationRoot: "D:\\Portable" };

    await vi.waitFor(() => {
      expect(result.settingsDraft.value.installationRoot).toBe("D:\\Portable");
      expect(result.previewResolvedSettings.value.serverExecutablePath).toBe("D:\\Portable\\custom-server.exe");
    });
    expect(previewResolvedSettings).toHaveBeenLastCalledWith(expect.objectContaining({ installationRoot: "D:\\Portable" }));
  });

  test("falls back to current resolved settings when preview fails", async () => {
    installDesktopApi(createDesktopApi({
      previewResolvedSettings: vi.fn(async () => {
        throw new Error("preview failed");
      }),
    }));

    const { result } = runInScope(() => useLauncherSettingsState(snapshot, ref(true)));
    result.editingDraft.value = { ...snapshot.value.launcher.settings, installationRoot: "E:\\Broken" };

    await vi.waitFor(() => {
      expect(window.rayleaLauncher.previewResolvedSettings).toHaveBeenCalled();
      expect(result.previewResolvedSettings.value).toEqual(snapshot.value.launcher.resolvedSettings);
    });
  });
});

describe("useLauncherConfirmations", () => {
  test.each(["close", "external-stop"] as const)("does not reopen a resolved %s prompt when the pending lookup arrives late", async (kind) => {
    let showClose!: () => void;
    let showExternal!: () => void;
    let resolvePending!: (pending: boolean) => void;
    const pending = new Promise<boolean>((resolve) => { resolvePending = resolve; });
    const unsubscribeClose = vi.fn();
    const unsubscribeExternal = vi.fn();
    const api = createDesktopApi({
      onShowExitConfirm: (listener: () => void) => { showClose = listener; return unsubscribeClose; },
      onShowExternalStopConfirm: (listener: () => void) => { showExternal = listener; return unsubscribeExternal; },
      hasPendingCloseConfirm: () => kind === "close" ? pending : Promise.resolve(false),
      hasPendingExternalStopConfirm: () => kind === "external-stop" ? pending : Promise.resolve(false),
    });
    installDesktopApi(api);

    const { result, stop } = runInScope(() => useLauncherConfirmations(vi.fn(), vi.fn()));
    if (kind === "close") showClose();
    else showExternal();
    expect(kind === "close" ? result.exitConfirmOpen.value : result.confirmedAction.value === "stop-external").toBe(true);

    if (kind === "close") result.handleExitConfirmClose();
    else result.handleConfirmedActionCancel();
    resolvePending(true);
    await flushPromises();

    expect(result.exitConfirmOpen.value).toBe(false);
    expect(result.confirmedAction.value).toBeNull();
    if (kind === "close") expect(api.closeConfirmResponse).toHaveBeenCalledWith({ action: "cancel", setAsDefault: false });
    else expect(api.externalStopConfirmResponse).toHaveBeenCalledWith(false);
    stop();
    expect(unsubscribeClose).toHaveBeenCalledOnce();
    expect(unsubscribeExternal).toHaveBeenCalledOnce();
  });
});

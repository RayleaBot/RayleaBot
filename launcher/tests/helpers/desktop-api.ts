import { vi } from "vitest";
import type { LauncherDesktopApi } from "@shared/desktop-api";
import type { LauncherSettings } from "@shared/launcher-models";

import { createLauncherSnapshot } from "./snapshot";

/** Derived paths the way the desktop layer resolves them under an installation root. */
export function previewSettings(settings: LauncherSettings) {
  return {
    installationRoot: settings.installationRoot,
    serverExecutablePath: settings.installationRoot ? `${settings.installationRoot}\\server\\raylea-server.exe` : "",
    configPath: settings.installationRoot ? `${settings.installationRoot}\\config\\user.yaml` : "",
    workdir: settings.installationRoot,
  };
}

/** A desktop bridge whose calls succeed without effects; tests override the calls they observe. */
export function createDesktopApi(overrides: Partial<LauncherDesktopApi> = {}): LauncherDesktopApi {
  return {
    getPlatform: vi.fn(async () => "win32-x64"),
    getSnapshot: vi.fn(async () => createLauncherSnapshot()),
    initialize: vi.fn(async () => undefined),
    refresh: vi.fn(async () => undefined),
    start: vi.fn(async () => undefined),
    stop: vi.fn(async () => undefined),
    restart: vi.fn(async () => undefined),
    resetAdmin: vi.fn(async () => undefined),
    checkForUpdates: vi.fn(async () => undefined),
    applyUpdate: vi.fn(async () => undefined),
    openWebUi: vi.fn(async () => undefined),
    openReleasePage: vi.fn(async () => undefined),
    openRepositoryPage: vi.fn(async () => undefined),
    openLogsDirectory: vi.fn(async () => undefined),
    saveSettings: vi.fn(async () => undefined),
    previewResolvedSettings: vi.fn(async (settings: LauncherSettings) => previewSettings(settings)),
    chooseInstallationRoot: vi.fn(async () => null),
    chooseServerExecutable: vi.fn(async () => null),
    chooseConfigFile: vi.fn(async () => null),
    chooseWorkdir: vi.fn(async () => null),
    exitApplication: vi.fn(async () => undefined),
    minimize: vi.fn(async () => undefined),
    maximize: vi.fn(async () => undefined),
    close: vi.fn(async () => undefined),
    hasPendingCloseConfirm: vi.fn(async () => false),
    closeConfirmResponse: vi.fn(async () => undefined),
    externalStopConfirmResponse: vi.fn(async () => undefined),
    hasPendingExternalStopConfirm: vi.fn(async () => false),
    setThemeMode: vi.fn(async () => undefined),
    isMaximized: vi.fn(async () => false),
    onSnapshot: vi.fn(() => () => undefined),
    onMaximizedChange: vi.fn(() => () => undefined),
    onShowExitConfirm: vi.fn(() => () => undefined),
    onShowExternalStopConfirm: vi.fn(() => () => undefined),
    ...overrides,
  };
}

export function installDesktopApi(api: LauncherDesktopApi) {
  Object.defineProperty(window, "rayleaLauncher", {
    configurable: true,
    value: api,
  });
}

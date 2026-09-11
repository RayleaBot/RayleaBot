import { Events } from "@wailsio/runtime";
import * as desktop from "../bindings/github.com/RayleaBot/RayleaBot/launcher/internal/desktop/service";
import * as desktopModels from "../bindings/github.com/RayleaBot/RayleaBot/launcher/internal/desktop/models";
import type { LauncherDesktopApi } from "../../shared/desktop-api";
import type {
  LauncherCloseConfirmResponse,
  LauncherSettings,
  LauncherSnapshot,
} from "../../shared/launcher-models";
import type { LauncherThemeMode } from "../../shared/launcher-theme";

function subscribe<T>(eventName: string, listener: (value: T) => void) {
  return Events.On(eventName, (event) => listener(event.data as T));
}

function expectEnumValue<Values extends Record<string, string>>(
  value: string, allowed: Values, field: string,
): Exclude<`${Values[keyof Values]}`, ""> {
  if (value !== "" && Object.values(allowed).includes(value)) {
    return value as Exclude<`${Values[keyof Values]}`, "">;
  }
  throw new Error(`Invalid Wails payload field: ${field}`);
}

function normalizeEnvironmentCheck(check: desktopModels.EnvironmentCheckResult) {
  return {
    ...check,
    scope: expectEnumValue(check.scope, desktopModels.EnvironmentCheckScope, "environmentChecks.scope"),
    severity: expectEnumValue(check.severity, desktopModels.CheckSeverity, "environmentChecks.severity"),
  };
}

export function normalizeWailsSnapshot(snapshot: desktopModels.LauncherSnapshot): LauncherSnapshot {
  const local = snapshot.launcher;
  return {
    ...snapshot,
    server: snapshot.server as LauncherSnapshot["server"],
    launcher: {
      ...local,
      processLifecycle: expectEnumValue(local.processLifecycle, desktopModels.LauncherProcessLifecycle, "launcher.processLifecycle"),
      processOwnership: expectEnumValue(local.processOwnership, desktopModels.LauncherProcessOwnership, "launcher.processOwnership"),
      environmentChecks: (local.environmentChecks ?? []).map(normalizeEnvironmentCheck),
      preflightChecks: (local.preflightChecks ?? []).map(normalizeEnvironmentCheck),
      advisoryChecks: (local.advisoryChecks ?? []).map(normalizeEnvironmentCheck),
      recentStderr: local.recentStderr ?? [],
      runtimePrepare: local.runtimePrepare ? {
        ...local.runtimePrepare,
        resources: (local.runtimePrepare.resources ?? []).map(resource => ({
          ...resource,
          status: expectEnumValue(resource.status, desktopModels.RuntimePrepareStatus, "runtimePrepare.resources.status"),
        })),
      } : null,
      releaseCheck: {
        ...local.releaseCheck,
        status: expectEnumValue(local.releaseCheck.status, desktopModels.ReleaseCheckStatus, "launcher.releaseCheck.status"),
      },
      settings: {
        ...local.settings,
        closeBehavior: expectEnumValue(local.settings.closeBehavior, desktopModels.LauncherCloseBehavior, "launcher.settings.closeBehavior"),
      },
      localRecoverySummary: local.localRecoverySummary as LauncherSnapshot["launcher"]["localRecoverySummary"],
    },
  };
}

export function createWailsDesktopApi(): LauncherDesktopApi {
  return {
    getPlatform: () => desktop.GetPlatform(),
    getSnapshot: async () => normalizeWailsSnapshot(await desktop.GetSnapshot()),
    initialize: () => desktop.Initialize(),
    refresh: () => desktop.Refresh(),
    start: () => desktop.Start(),
    stop: () => desktop.Stop(),
    resetAdmin: () => desktop.ResetAdmin(),
    checkForUpdates: () => desktop.CheckForUpdates(),
    downloadUpdate: () => desktop.DownloadUpdate(),
    installDownloadedUpdate: () => desktop.InstallDownloadedUpdate(),
    openWebUi: (targetPath = "") => desktop.OpenWebUI(targetPath),
    openReleasePage: () => desktop.OpenReleasePage(),
    openRepositoryPage: () => desktop.OpenRepositoryPage(),
    openLogsDirectory: () => desktop.OpenLogsDirectory(),
    saveSettings: (settings: LauncherSettings) => desktop.SaveSettings(settings as desktopModels.LauncherSettings),
    previewResolvedSettings: (settings: LauncherSettings) =>
      desktop.PreviewResolvedSettings(settings as desktopModels.LauncherSettings),
    chooseInstallationRoot: () => desktop.ChooseInstallationRoot(),
    chooseServerExecutable: () => desktop.ChooseServerExecutable(),
    chooseConfigFile: () => desktop.ChooseConfigFile(),
    chooseWorkdir: () => desktop.ChooseWorkdir(),
    exitApplication: () => desktop.ExitApplication(),
    minimize: () => desktop.Minimize(),
    maximize: () => desktop.Maximize(),
    close: () => desktop.Close(),
    hasPendingCloseConfirm: () => desktop.HasPendingCloseConfirm(),
    closeConfirmResponse: (response: LauncherCloseConfirmResponse) =>
      desktop.CloseConfirmResponse(response as desktopModels.LauncherCloseConfirmResponse),
    externalStopConfirmResponse: (confirmed: boolean) => desktop.ExternalStopConfirmResponse(confirmed),
    hasPendingExternalStopConfirm: () => desktop.HasPendingExternalStopConfirm(),
    setThemeMode: (mode: LauncherThemeMode) => desktop.SetThemeMode(mode),
    isMaximized: () => desktop.IsMaximized(),
    onSnapshot: (listener) => Events.On("launcher:snapshot", (event) => {
      listener(normalizeWailsSnapshot(event.data as desktopModels.LauncherSnapshot));
    }),
    onMaximizedChange: (listener) => subscribe<boolean>("launcher:maximized-change", listener),
    onShowExitConfirm: (listener) => Events.On("launcher:show-exit-confirm", () => listener()),
    onShowExternalStopConfirm: (listener) =>
      Events.On("launcher:show-external-stop-confirm", () => listener()),
  };
}

export function installWailsDesktopApi() {
  const previous = window.rayleaLauncher;
  const installed = createWailsDesktopApi();
  window.rayleaLauncher = installed;
  return () => {
    if (window.rayleaLauncher !== installed) {
      return;
    }
    if (previous) {
      window.rayleaLauncher = previous;
    } else {
      Reflect.deleteProperty(window, "rayleaLauncher");
    }
  };
}

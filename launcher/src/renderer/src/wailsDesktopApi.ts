import { Events } from "@wailsio/runtime";
import * as desktop from "../bindings/github.com/RayleaBot/RayleaBot/launcher/internal/desktop/service";
import type * as desktopModels from "../bindings/github.com/RayleaBot/RayleaBot/launcher/internal/desktop/models";
import type { LauncherDesktopApi } from "../../shared/desktop-api";
import type {
  LauncherCloseConfirmResponse,
  LauncherResolvedSettings,
  LauncherSettings,
  LauncherSnapshot,
} from "../../shared/launcher-models";
import type { LauncherThemeMode } from "../../shared/launcher-theme";
import {
  normalizeHealthPayload,
  normalizeReadinessPayload,
  normalizeRecoverySummaryPayload,
  normalizeSystemStatusPayload,
} from "../../shared/server-payload-validation";

function asDesktopSettings(settings: LauncherSettings): desktopModels.LauncherSettings {
  return {
    installationRoot: settings.installationRoot,
    closeBehavior: settings.closeBehavior,
    advancedOverrides: settings.advancedOverrides
      ? {
          serverExecutablePath: settings.advancedOverrides.serverExecutablePath,
          configPath: settings.advancedOverrides.configPath,
          workdir: settings.advancedOverrides.workdir,
        }
      : undefined,
  };
}

function subscribe<T>(eventName: string, listener: (value: T) => void) {
  return Events.On(eventName, (event) => listener(event.data as T));
}

function expectEnumValue<const Values extends readonly string[]>(
  value: string,
  allowed: Values,
  field: string,
): Values[number] {
  if ((allowed as readonly string[]).includes(value)) {
    return value as Values[number];
  }
  throw new Error(`Invalid Wails payload field: ${field}`);
}

function normalizeEnvironmentCheck(
  check: desktopModels.EnvironmentCheckResult,
): LauncherSnapshot["launcher"]["environmentChecks"][number] {
  return {
    scope: expectEnumValue(check.scope, ["preflight", "advisory"] as const, "environmentChecks.scope"),
    code: check.code,
    title: check.title,
    severity: expectEnumValue(check.severity, ["ok", "warning", "error"] as const, "environmentChecks.severity"),
    summary: check.summary,
    detail: check.detail,
    remediation: check.remediation,
  };
}

function normalizeResolvedSettings(
  settings: desktopModels.LauncherResolvedSettings,
): LauncherResolvedSettings {
  return {
    installationRoot: settings.installationRoot,
    serverExecutablePath: settings.serverExecutablePath,
    configPath: settings.configPath,
    workdir: settings.workdir,
  };
}

export function normalizeWailsSnapshot(snapshot: desktopModels.LauncherSnapshot): LauncherSnapshot {
  const runtimePrepare: LauncherSnapshot["launcher"]["runtimePrepare"] = snapshot.launcher.runtimePrepare
    ? {
        active: snapshot.launcher.runtimePrepare.active,
        currentKind: snapshot.launcher.runtimePrepare.currentKind,
        summary: snapshot.launcher.runtimePrepare.summary,
        resources: (snapshot.launcher.runtimePrepare.resources ?? []).map((resource) => ({
          kind: resource.kind,
          label: resource.label,
          resourceId: resource.resourceId,
          version: resource.version,
          sourceLabel: resource.sourceLabel,
          sourceUrl: resource.sourceUrl,
          archivePath: resource.archivePath,
          storeRoot: resource.storeRoot,
          stage: resource.stage,
          status: expectEnumValue(resource.status, ["pending", "running", "succeeded", "failed"] as const, "runtimePrepare.resources.status"),
          progress: resource.progress,
          downloadedBytes: resource.downloadedBytes,
          totalBytes: resource.totalBytes,
          extractedEntries: resource.extractedEntries,
          totalEntries: resource.totalEntries,
          summary: resource.summary,
          error: resource.error,
          updatedAt: resource.updatedAt,
        })),
      }
    : null;

  const server: LauncherSnapshot["server"] = {
    health: normalizeHealthPayload(snapshot.server.health),
    readiness: normalizeReadinessPayload(snapshot.server.readiness),
    systemStatus: normalizeSystemStatusPayload(snapshot.server.systemStatus),
  };
  const launcher: LauncherSnapshot["launcher"] = {
    processId: snapshot.launcher.processId,
    processLifecycle: expectEnumValue(snapshot.launcher.processLifecycle, ["stopped", "starting", "running", "stopping"] as const, "launcher.processLifecycle"),
    processOwnership: expectEnumValue(snapshot.launcher.processOwnership, ["none", "launcher_managed", "external"] as const, "launcher.processOwnership"),
    environmentChecks: (snapshot.launcher.environmentChecks ?? []).map(normalizeEnvironmentCheck),
    preflightChecks: (snapshot.launcher.preflightChecks ?? []).map(normalizeEnvironmentCheck),
    advisoryChecks: (snapshot.launcher.advisoryChecks ?? []).map(normalizeEnvironmentCheck),
    recentStderr: snapshot.launcher.recentStderr ?? [],
    runtimePrepare,
    releaseCheck: {
      status: expectEnumValue(snapshot.launcher.releaseCheck.status, [
        "disabled",
        "idle",
        "checking",
        "up_to_date",
        "update_available",
        "downloading",
        "ready_to_install",
        "installing",
        "succeeded",
        "failed",
        "rolled_back",
        "rollback_failed",
      ] as const, "launcher.releaseCheck.status"),
      currentVersion: snapshot.launcher.releaseCheck.currentVersion,
      latestVersion: snapshot.launcher.releaseCheck.latestVersion,
      summary: snapshot.launcher.releaseCheck.summary,
      detail: snapshot.launcher.releaseCheck.detail,
      errorCode: snapshot.launcher.releaseCheck.errorCode,
      releasePageUrl: snapshot.launcher.releaseCheck.releasePageUrl,
      updateAvailable: snapshot.launcher.releaseCheck.updateAvailable,
      downloadProgress: snapshot.launcher.releaseCheck.downloadProgress,
      downloadedBytes: snapshot.launcher.releaseCheck.downloadedBytes,
      totalBytes: snapshot.launcher.releaseCheck.totalBytes,
      artifactFileName: snapshot.launcher.releaseCheck.artifactFileName,
      canCheck: snapshot.launcher.releaseCheck.canCheck,
      canDownload: snapshot.launcher.releaseCheck.canDownload,
      canInstall: snapshot.launcher.releaseCheck.canInstall,
    },
    lastLocalError: snapshot.launcher.lastLocalError,
    statusHint: snapshot.launcher.statusHint,
    settings: {
      installationRoot: snapshot.launcher.settings.installationRoot,
      closeBehavior: expectEnumValue(snapshot.launcher.settings.closeBehavior, ["ask_every_time", "hide_to_tray", "exit_application"] as const, "launcher.settings.closeBehavior"),
      advancedOverrides: snapshot.launcher.settings.advancedOverrides
        ? {
            serverExecutablePath: snapshot.launcher.settings.advancedOverrides.serverExecutablePath,
            configPath: snapshot.launcher.settings.advancedOverrides.configPath,
            workdir: snapshot.launcher.settings.advancedOverrides.workdir,
          }
        : undefined,
    },
    resolvedSettings: normalizeResolvedSettings(snapshot.launcher.resolvedSettings),
    endpoint: {
      host: snapshot.launcher.endpoint.host,
      port: snapshot.launcher.endpoint.port,
      baseUrl: snapshot.launcher.endpoint.baseUrl,
    },
    localRecoverySummary: normalizeRecoverySummaryPayload(snapshot.launcher.localRecoverySummary),
  };

  return { server, launcher };
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
    saveSettings: (settings: LauncherSettings) => desktop.SaveSettings(asDesktopSettings(settings)),
    previewResolvedSettings: async (settings: LauncherSettings) =>
      normalizeResolvedSettings(await desktop.PreviewResolvedSettings(asDesktopSettings(settings))),
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
      desktop.CloseConfirmResponse({ action: response.action, setAsDefault: response.setAsDefault }),
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

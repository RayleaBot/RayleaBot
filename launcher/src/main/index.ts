import path from "node:path";
import { app, BrowserWindow, dialog, ipcMain, Menu, Tray, nativeTheme, protocol } from "electron";
import type { LauncherSettings, LauncherSnapshot, TrayMenuEntry, TrayMenuState } from "../shared/launcher-models";
import { deriveLauncherPresentation } from "../shared/launcher-presentation";
import { launcherCopy } from "../shared/launcher-copy";
import { launcherEventChannels, launcherInvokeChannels } from "../shared/launcher-ipc";
import {
  parseLauncherCloseConfirmResponse,
  parseLauncherSettingsInput,
  parseLauncherThemeMode,
  sanitizeLauncherWebTargetPath,
} from "../shared/launcher-validation";
import { resolveLauncherWindowBackground } from "../shared/launcher-theme";
import { createLauncherCoordinator } from "./services/launcher-coordinator";
import { inspectEnvironmentFromNode } from "./services/environment";
import { JsonLauncherSettingsStore, resolveLauncherSettings } from "./services/settings-store";
import { resolveServerEndpoint } from "./services/endpoint-resolver";
import { FetchLauncherManagementClient } from "./services/management-client";
import { ServerProcessController } from "./services/process-controller";
import { LauncherServerCredentials } from "./services/server-credentials";
import { isEndpointListening, tryStopEndpointProcess } from "./services/port-process";
import { externalOpener } from "./services/external-opener";
import { LauncherReleaseFeedClient } from "./services/release-feed";
import { NodeConfigInitializer } from "./services/config-initializer";
import { NodeResetAdminRunner } from "./services/reset-admin-runner";
import { buildTrayMenuEntries } from "./services/tray-menu";
import { createApplicationExitManager } from "./services/app-exit";
import { resolveLauncherAssetPaths, resolveLauncherBasePath } from "./services/app-paths";
import { NodeRecoverySummaryReader } from "./services/recovery-summary-reader";
import { applyLauncherThemeMode, syncLauncherWindowBackground } from "./services/launcher-theme";
import { createTrayImage } from "./services/tray-icon";
import { wireSingleInstanceLifecycle } from "./services/single-instance";
import {
  LAUNCHER_RENDERER_SCHEME,
  createSecureIpcRegistrar,
  denyRendererPermissions,
  installRendererNavigationGuards,
  launcherRendererContentSecurityPolicy,
  resolveLauncherRendererTarget,
  wirePackagedRendererProtocol,
} from "./services/electron-security";
import {
  completeUpdateHeartbeat,
  consumeUpdateHeartbeatEnvironment,
  launchInterruptedUpdateRecovery,
} from "./services/update-resume";

protocol.registerSchemesAsPrivileged([
  {
    scheme: LAUNCHER_RENDERER_SCHEME,
    privileges: {
      allowServiceWorkers: false,
      bypassCSP: false,
      codeCache: true,
      corsEnabled: false,
      secure: true,
      standard: true,
      stream: false,
      supportFetchAPI: false,
    },
  },
]);
app.enableSandbox();

const rendererTarget = resolveLauncherRendererTarget({
  isPackaged: app.isPackaged,
  devServerUrl: process.env.RAYLEA_DEV_SERVER_URL,
});

let mainWindow: BrowserWindow | null = null;
let tray: Tray | null = null;
let windowMaximized = false;

const executableBasePath = resolveLauncherBasePath({
  appPath: app.getAppPath(),
  executablePath: app.getPath("exe"),
  isPackaged: app.isPackaged,
});
const updateHeartbeatEnvironmentPresent = Boolean(
  process.env.RAYLEA_UPDATE_HEARTBEAT || process.env.RAYLEA_UPDATE_TOKEN,
);
const updateHeartbeatRequest = consumeUpdateHeartbeatEnvironment();
const settingsStore = new JsonLauncherSettingsStore(executableBasePath, process.platform);
const serverCredentials = new LauncherServerCredentials();
const processController = new ServerProcessController({ credentials: serverCredentials });
const coordinator = createLauncherCoordinator({
  settingsStore,
  endpointResolver: { resolve: resolveServerEndpoint },
  inspectEnvironment: inspectEnvironmentFromNode,
  managementClient: new FetchLauncherManagementClient({
    getLauncherControlToken: () => serverCredentials.controlToken,
  }),
  processController,
  isEndpointListening,
  tryStopEndpointProcess,
  externalOpener,
  releaseFeedClient: new LauncherReleaseFeedClient(executableBasePath),
  configInitializer: new NodeConfigInitializer(),
  resetAdminRunner: new NodeResetAdminRunner(),
  recoverySummaryReader: new NodeRecoverySummaryReader(),
  confirmExternalServiceStop: async () => {
    const result = await dialog.showMessageBox(mainWindow!, {
      type: "warning",
      title: "停止现有服务",
      message: "检测到的现有服务并非由当前 Launcher 启动。",
      detail: "确认后，Launcher 会尝试通过正式管理接口请求该服务停止。",
      buttons: ["继续停止", "取消"],
      cancelId: 1,
      defaultId: 1,
    });
    return result.response === 0;
  },
});
const appExitManager = createApplicationExitManager({
  isManagedProcessRunning: () => processController.isRunning,
  stopManagedProcess: () => coordinator.stop(),
  forceKillManagedProcess: () => processController.forceKill(),
  quitApplication: () => app.quit(),
});

function trayStateFromSnapshot(snapshot: LauncherSnapshot): TrayMenuState {
  const presentation = deriveLauncherPresentation(snapshot);

  return {
    trayStatusSummary: launcherCopy.statusSummary(presentation.state),
    canOpenWebUi: presentation.canOpenWebUi,
    trayServiceAction: presentation.canStopService ? "stop" : "start",
    trayServiceActionLabel: presentation.canStopService ? "停止服务" : "启动服务",
    canRunTrayServiceAction: presentation.canRunServiceAction,
  };
}

async function chooseServerExecutable() {
  const filters =
    process.platform === "win32"
      ? [{ name: "Raylea Server", extensions: ["exe"] }]
      : [{ name: "Raylea Server", extensions: [""] }];
  const result = await dialog.showOpenDialog(mainWindow!, {
    properties: ["openFile"],
    filters,
  });
  return result.canceled ? null : result.filePaths[0] ?? null;
}

async function chooseConfigFile() {
  const result = await dialog.showOpenDialog(mainWindow!, {
    properties: ["openFile"],
    filters: [{ name: "YAML", extensions: ["yaml", "yml"] }],
  });
  return result.canceled ? null : result.filePaths[0] ?? null;
}

async function chooseInstallationRoot() {
  const result = await dialog.showOpenDialog(mainWindow!, {
    properties: ["openDirectory", "createDirectory"],
  });
  return result.canceled ? null : result.filePaths[0] ?? null;
}

async function chooseWorkdir() {
  const result = await dialog.showOpenDialog(mainWindow!, {
    properties: ["openDirectory", "createDirectory"],
  });
  return result.canceled ? null : result.filePaths[0] ?? null;
}

async function handleCloseRequest() {
  const snapshot = coordinator.snapshot;
  if (snapshot.launcher.settings.closeBehavior === "hide_to_tray") {
    mainWindow?.hide();
    return;
  }

  if (snapshot.launcher.settings.closeBehavior === "exit_application") {
    await appExitManager.requestExit();
    return;
  }

  mainWindow?.webContents.send(launcherEventChannels.showExitConfirm);
}

async function runTrayAction(action: TrayMenuEntry["action"]) {
  switch (action) {
    case "restore":
      mainWindow?.show();
      mainWindow?.focus();
      break;
    case "open_web":
      await coordinator.openWebUi();
      break;
    case "start":
      await coordinator.start();
      break;
    case "stop":
      await coordinator.stop();
      break;
    case "open_logs":
      await coordinator.openLogsDirectory();
      break;
    case "exit":
      await appExitManager.requestExit();
      break;
    default:
      break;
  }
}

function refreshTrayMenu(snapshot: LauncherSnapshot) {
  if (!tray) {
    return;
  }
  const entries = buildTrayMenuEntries(trayStateFromSnapshot(snapshot));
  const menu = Menu.buildFromTemplate(
    entries.map((entry) => {
      if (entry.action === "separator") {
        return { type: "separator" as const };
      }
      return {
        label: entry.label,
        enabled: entry.enabled,
        click: () => void runTrayAction(entry.action),
      };
    }),
  );
  const presentation = deriveLauncherPresentation(snapshot);
  tray.setToolTip(`RayleaBot 启动器 · ${launcherCopy.statusSummary(presentation.state)}`);
  tray.setContextMenu(menu);
}

async function createMainWindow() {
  nativeTheme.themeSource = "system";
  const isDark = nativeTheme.shouldUseDarkColors;
  const { preloadPath } = resolveLauncherAssetPaths(app.getAppPath());

  mainWindow = new BrowserWindow({
    width: 1380,
    height: 920,
    minWidth: 760,
    minHeight: 560,
    title: "RayleaBot 启动器",
    frame: false,
    roundedCorners: true,
    backgroundColor: resolveLauncherWindowBackground(isDark ? "dark" : "light"),
    show: false,
    autoHideMenuBar: true,
    webPreferences: {
      allowRunningInsecureContent: false,
      devTools: !app.isPackaged,
      navigateOnDragDrop: false,
      nodeIntegration: false,
      preload: preloadPath,
      contextIsolation: true,
      sandbox: true,
      safeDialogs: true,
      webSecurity: true,
      webviewTag: false,
    },
  });

  installRendererNavigationGuards(mainWindow.webContents);
  denyRendererPermissions(mainWindow.webContents.session);

  if (rendererTarget.kind === "development") {
    const contentSecurityPolicy = launcherRendererContentSecurityPolicy(rendererTarget);
    mainWindow.webContents.session.webRequest.onHeadersReceived(
      { urls: [`${rendererTarget.origin}/*`] },
      (details, callback) => {
        callback({
          responseHeaders: {
            ...details.responseHeaders,
            "Content-Security-Policy": [contentSecurityPolicy],
            "Cross-Origin-Opener-Policy": ["same-origin"],
            "Permissions-Policy": [
              "accelerometer=(), camera=(), geolocation=(), microphone=(), payment=(), usb=()",
            ],
            "Referrer-Policy": ["no-referrer"],
            "X-Content-Type-Options": ["nosniff"],
          },
        });
      },
    );
  }

  mainWindow.on("ready-to-show", () => {
    mainWindow?.show();
  });

  mainWindow.on("maximize", () => {
    windowMaximized = true;
    mainWindow?.webContents.send(launcherEventChannels.maximizedChange, true);
  });

  mainWindow.on("unmaximize", () => {
    windowMaximized = false;
    mainWindow?.webContents.send(launcherEventChannels.maximizedChange, false);
  });

  mainWindow.on("close", (event) => {
    if (appExitManager.shouldAllowQuit()) {
      return;
    }
    event.preventDefault();
    void handleCloseRequest();
  });

  await mainWindow.loadURL(rendererTarget.url);
}

function wireIpc() {
  const secureIpc = createSecureIpcRegistrar({
    ipcMain,
    expectedOrigin: rendererTarget.origin,
    getMainWebContents: () => mainWindow?.webContents ?? null,
  });

  secureIpc.noArgs(launcherInvokeChannels.minimize, () => mainWindow?.minimize());
  secureIpc.noArgs(launcherInvokeChannels.maximize, () => {
    if (!mainWindow) return;
    if (windowMaximized) {
      mainWindow.unmaximize();
    } else {
      mainWindow.maximize();
    }
  });
  secureIpc.noArgs(launcherInvokeChannels.close, () => handleCloseRequest());
  secureIpc.noArgs(launcherInvokeChannels.isMaximized, () => windowMaximized);
  secureIpc.noArgs(launcherInvokeChannels.getPlatform, () => `${process.platform}-${process.arch}`);
  secureIpc.noArgs(launcherInvokeChannels.getSnapshot, () => coordinator.snapshot);
  secureIpc.noArgs(launcherInvokeChannels.initialize, () => coordinator.initialize());
  secureIpc.noArgs(launcherInvokeChannels.refresh, () => coordinator.refresh());
  secureIpc.noArgs(launcherInvokeChannels.retry, () => coordinator.retry());
  secureIpc.noArgs(launcherInvokeChannels.start, () => coordinator.start());
  secureIpc.noArgs(launcherInvokeChannels.stop, () => coordinator.stop());
  secureIpc.noArgs(launcherInvokeChannels.resetAdmin, () => coordinator.resetAdmin());
  secureIpc.noArgs(launcherInvokeChannels.checkForUpdates, () => coordinator.checkForUpdates());
  secureIpc.noArgs(launcherInvokeChannels.downloadUpdate, () => coordinator.downloadUpdate());
  secureIpc.noArgs(launcherInvokeChannels.installDownloadedUpdate, async () => {
    await coordinator.prepareUpdateInstall(process.pid);
    await appExitManager.requestExit();
  });
  secureIpc.oneArg(
    launcherInvokeChannels.openWeb,
    sanitizeLauncherWebTargetPath,
    (targetPath) => coordinator.openWebUi(targetPath),
  );
  secureIpc.noArgs(launcherInvokeChannels.openReleasePage, () => coordinator.openReleasePage());
  secureIpc.noArgs(launcherInvokeChannels.openRepositoryPage, () => coordinator.openRepositoryPage());
  secureIpc.noArgs(launcherInvokeChannels.openLogs, () => coordinator.openLogsDirectory());
  secureIpc.oneArg(
    launcherInvokeChannels.saveSettings,
    parseLauncherSettingsInput,
    (settings) => coordinator.saveSettings(settings),
  );
  secureIpc.oneArg(
    launcherInvokeChannels.previewResolvedSettings,
    parseLauncherSettingsInput,
    (settings) => resolveLauncherSettings(settings, process.platform),
  );
  secureIpc.noArgs(launcherInvokeChannels.chooseInstallationRoot, () => chooseInstallationRoot());
  secureIpc.noArgs(launcherInvokeChannels.chooseServer, () => chooseServerExecutable());
  secureIpc.noArgs(launcherInvokeChannels.chooseConfig, () => chooseConfigFile());
  secureIpc.noArgs(launcherInvokeChannels.chooseWorkdir, () => chooseWorkdir());
  secureIpc.oneArg(launcherInvokeChannels.closeConfirmResponse, parseLauncherCloseConfirmResponse, async (response) => {
    if (response.action === "cancel") {
      return;
    }

    const snapshot = coordinator.snapshot;
    if (response.setAsDefault) {
      const nextBehavior = response.action === "hide" ? "hide_to_tray" : "exit_application";
      await coordinator.saveSettings({
        ...snapshot.launcher.settings,
        closeBehavior: nextBehavior,
      } satisfies LauncherSettings);
    }

    if (response.action === "hide") {
      mainWindow?.hide();
    } else {
      await appExitManager.requestExit();
    }
  });
  secureIpc.oneArg(launcherInvokeChannels.setThemeMode, parseLauncherThemeMode, (mode) => {
    applyLauncherThemeMode(nativeTheme, mainWindow, mode);
  });
  secureIpc.noArgs(launcherInvokeChannels.exit, () => appExitManager.requestExit());
}

async function bootstrap() {
  await app.whenReady();
  if (!updateHeartbeatEnvironmentPresent && await launchInterruptedUpdateRecovery(executableBasePath, process.pid)) {
    await appExitManager.requestExit();
    return;
  }
  if (rendererTarget.kind === "packaged") {
    const { rendererPath } = resolveLauncherAssetPaths(app.getAppPath());
    wirePackagedRendererProtocol({ protocol, rendererRoot: path.dirname(rendererPath) });
  }
  wireIpc();
  await createMainWindow();
  nativeTheme.on("updated", () => syncLauncherWindowBackground(nativeTheme, mainWindow));

  tray = new Tray(createTrayImage());
  tray.on("click", () => {
    if (mainWindow?.isVisible()) {
      mainWindow.hide();
    } else {
      mainWindow?.show();
      mainWindow?.focus();
    }
  });

  coordinator.subscribe((snapshot) => {
    refreshTrayMenu(snapshot);
    mainWindow?.webContents.send(launcherEventChannels.snapshot, snapshot);
  });

  await coordinator.initialize();
  await completeUpdateHeartbeat(executableBasePath, updateHeartbeatRequest, {
    startService: () => coordinator.start(),
    getSnapshot: () => coordinator.snapshot,
    launcherPid: process.pid,
    servicePid: () => processController.processId,
  });
}

app.on("window-all-closed", () => {
  if (appExitManager.shouldAllowQuit()) {
    app.quit();
  }
});

app.on("before-quit", (event) => {
  if (appExitManager.shouldAllowQuit()) {
    return;
  }
  event.preventDefault();
  void appExitManager.requestExit();
});

if (wireSingleInstanceLifecycle(app, () => mainWindow)) {
  void bootstrap();
}

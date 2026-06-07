import { app, BrowserWindow, dialog, ipcMain, Menu, Tray, nativeTheme } from "electron";
import type { LauncherSettings, LauncherSnapshot, TrayMenuEntry, TrayMenuState } from "../shared/launcher-models";
import { deriveLauncherPresentation } from "../shared/launcher-presentation";
import { launcherCopy } from "../shared/launcher-copy";
import { launcherEventChannels, launcherInvokeChannels } from "../shared/launcher-ipc";
import {
  parseLauncherSettingsInput,
  sanitizeLauncherWebTargetPath,
} from "../shared/launcher-validation";
import { createLauncherCoordinator } from "./services/launcher-coordinator";
import { inspectEnvironmentFromNode } from "./services/environment";
import { JsonLauncherSettingsStore, resolveLauncherSettings } from "./services/settings-store";
import { resolveServerEndpoint } from "./services/endpoint-resolver";
import { FetchLauncherManagementClient } from "./services/management-client";
import { ServerProcessController } from "./services/process-controller";
import { isEndpointListening, tryStopEndpointProcess } from "./services/port-process";
import { externalOpener } from "./services/external-opener";
import { LauncherReleaseFeedClient } from "./services/release-feed";
import { NodeResetAdminRunner } from "./services/reset-admin-runner";
import { buildTrayMenuEntries } from "./services/tray-menu";
import { createApplicationExitManager } from "./services/app-exit";
import { resolveLauncherAssetPaths, resolveLauncherBasePath } from "./services/app-paths";
import { NodeRecoverySummaryReader } from "./services/recovery-summary-reader";
import { createTrayImage } from "./services/tray-icon";
import { wireSingleInstanceLifecycle } from "./services/single-instance";

const devServerUrl = process.env.RAYLEA_DEV_SERVER_URL;

let mainWindow: BrowserWindow | null = null;
let tray: Tray | null = null;
let windowMaximized = false;

const executableBasePath = resolveLauncherBasePath({
  appPath: app.getAppPath(),
  executablePath: app.getPath("exe"),
  isPackaged: app.isPackaged,
});
const settingsStore = new JsonLauncherSettingsStore(executableBasePath, process.platform);
const processController = new ServerProcessController();
const coordinator = createLauncherCoordinator({
  settingsStore,
  endpointResolver: { resolve: resolveServerEndpoint },
  inspectEnvironment: inspectEnvironmentFromNode,
  managementClient: new FetchLauncherManagementClient(),
  processController,
  isEndpointListening,
  tryStopEndpointProcess,
  externalOpener,
  releaseFeedClient: new LauncherReleaseFeedClient(executableBasePath),
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
  const { preloadPath, rendererPath } = resolveLauncherAssetPaths(app.getAppPath());

  mainWindow = new BrowserWindow({
    width: 1380,
    height: 920,
    minWidth: 1120,
    minHeight: 760,
    title: "RayleaBot 启动器",
    frame: false,
    roundedCorners: true,
    backgroundColor: isDark ? "#0f172a" : "#f8fafc",
    show: false,
    autoHideMenuBar: true,
    webPreferences: {
      preload: preloadPath,
      contextIsolation: true,
      sandbox: false,
    },
  });

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

  if (devServerUrl) {
    await mainWindow.loadURL(devServerUrl);
  } else {
    await mainWindow.loadFile(rendererPath);
  }
}

function wireIpc() {
  ipcMain.handle(launcherInvokeChannels.minimize, () => mainWindow?.minimize());
  ipcMain.handle(launcherInvokeChannels.maximize, () => {
    if (!mainWindow) return;
    if (windowMaximized) {
      mainWindow.unmaximize();
    } else {
      mainWindow.maximize();
    }
  });
  ipcMain.handle(launcherInvokeChannels.close, () => handleCloseRequest());
  ipcMain.handle(launcherInvokeChannels.isMaximized, () => windowMaximized);
  ipcMain.handle(launcherInvokeChannels.getPlatform, async () => `${process.platform}-${process.arch}`);
  ipcMain.handle(launcherInvokeChannels.getSnapshot, async () => coordinator.snapshot);
  ipcMain.handle(launcherInvokeChannels.initialize, async () => coordinator.initialize());
  ipcMain.handle(launcherInvokeChannels.refresh, async () => coordinator.refresh());
  ipcMain.handle(launcherInvokeChannels.retry, async () => coordinator.retry());
  ipcMain.handle(launcherInvokeChannels.start, async () => coordinator.start());
  ipcMain.handle(launcherInvokeChannels.stop, async () => coordinator.stop());
  ipcMain.handle(launcherInvokeChannels.resetAdmin, async () => coordinator.resetAdmin());
  ipcMain.handle(launcherInvokeChannels.openWeb, async (_event, targetPath?: string) =>
    coordinator.openWebUi(sanitizeLauncherWebTargetPath(targetPath)),
  );
  ipcMain.handle(launcherInvokeChannels.openReleasePage, async () => coordinator.openReleasePage());
  ipcMain.handle(launcherInvokeChannels.openLogs, async () => coordinator.openLogsDirectory());
  ipcMain.handle(launcherInvokeChannels.saveSettings, async (_event, settings: LauncherSettings) =>
    coordinator.saveSettings(parseLauncherSettingsInput(settings)),
  );
  ipcMain.handle(launcherInvokeChannels.previewResolvedSettings, async (_event, settings: LauncherSettings) =>
    resolveLauncherSettings(parseLauncherSettingsInput(settings), process.platform),
  );
  ipcMain.handle(launcherInvokeChannels.chooseInstallationRoot, async () => chooseInstallationRoot());
  ipcMain.handle(launcherInvokeChannels.chooseServer, async () => chooseServerExecutable());
  ipcMain.handle(launcherInvokeChannels.chooseConfig, async () => chooseConfigFile());
  ipcMain.handle(launcherInvokeChannels.chooseWorkdir, async () => chooseWorkdir());
  ipcMain.handle(launcherInvokeChannels.closeConfirmResponse, async (_event, response: { action: "hide" | "exit" | "cancel"; setAsDefault: boolean }) => {
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
  ipcMain.handle(launcherInvokeChannels.exit, async () => appExitManager.requestExit());
}

async function bootstrap() {
  await app.whenReady();
  wireIpc();
  await createMainWindow();

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

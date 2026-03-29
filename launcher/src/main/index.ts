import path from "node:path";
import { app, BrowserWindow, dialog, ipcMain, Menu, Tray, nativeImage, nativeTheme } from "electron";
import type { LauncherSettings, LauncherSnapshot, TrayMenuEntry, TrayMenuState } from "../shared/launcher-models";
import { launcherCopy } from "../shared/launcher-copy";
import { createLauncherCoordinator } from "./services/launcher-coordinator";
import { inspectEnvironmentFromNode } from "./services/environment";
import { JsonLauncherSettingsStore } from "./services/settings-store";
import { resolveServerEndpoint } from "./services/endpoint-resolver";
import { FetchLauncherManagementClient } from "./services/management-client";
import { ServerProcessController } from "./services/process-controller";
import { isEndpointListening, tryStopEndpointProcess } from "./services/port-process";
import { externalOpener } from "./services/external-opener";
import { LauncherReleaseFeedClient } from "./services/release-feed";
import { buildTrayMenuEntries } from "./services/tray-menu";

const devServerUrl = process.env.RAYLEA_DEV_SERVER_URL;

let mainWindow: BrowserWindow | null = null;
let tray: Tray | null = null;
let shouldQuit = false;
let windowMaximized = false;

const executableBasePath = app.isPackaged ? path.dirname(app.getPath("exe")) : path.resolve(__dirname, "..", "..", "..", "..");
const settingsStore = new JsonLauncherSettingsStore(app.getPath("userData"), executableBasePath, process.platform);
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
});

function trayStateFromSnapshot(snapshot: LauncherSnapshot): TrayMenuState {
  return {
    trayStatusSummary: launcherCopy.statusSummary(snapshot.serviceState),
    canOpenWebUi: snapshot.serviceState === "external_service" || snapshot.serviceState === "ready",
    trayServiceAction: snapshot.serviceState === "external_service" || snapshot.serviceState === "ready" || snapshot.serviceState === "failed" ? "stop" : "start",
    trayServiceActionLabel:
      snapshot.serviceState === "external_service" || snapshot.serviceState === "ready" || snapshot.serviceState === "failed" ? "停止服务" : "启动服务",
    canRunTrayServiceAction: snapshot.serviceState !== "starting" && snapshot.serviceState !== "shutting_down",
  };
}

function createTrayImage() {
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" width="64" height="64">
      <rect width="64" height="64" rx="18" fill="#122032"/>
      <path d="M18 18h28v28H18z" fill="#264763" rx="10"/>
      <path d="M24 22h16c4 0 8 4 8 8v12H36V30c0-2-2-4-4-4h-8z" fill="#7fd6ff"/>
      <circle cx="28" cy="42" r="6" fill="#d6f5ff"/>
    </svg>
  `;
  return nativeImage.createFromDataURL(`data:image/svg+xml;base64,${Buffer.from(svg).toString("base64")}`);
}

function resolvePreloadPath() {
  return path.resolve(__dirname, "..", "..", "preload", "preload", "index.js");
}

function resolveRendererPath() {
  return path.resolve(__dirname, "..", "..", "renderer", "index.html");
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

async function chooseWorkdir() {
  const result = await dialog.showOpenDialog(mainWindow!, {
    properties: ["openDirectory", "createDirectory"],
  });
  return result.canceled ? null : result.filePaths[0] ?? null;
}

async function handleCloseRequest() {
  const snapshot = coordinator.snapshot;
  if (snapshot.settings.closeBehavior === "hide_to_tray") {
    mainWindow?.hide();
    return;
  }

  if (snapshot.settings.closeBehavior === "exit_application") {
    shouldQuit = true;
    app.quit();
    return;
  }

  const result = await dialog.showMessageBox(mainWindow!, {
    type: "question",
    title: "关闭窗口",
    message: "关闭窗口时，选择保留到托盘或直接退出。",
    detail: "隐藏到托盘后，服务与常用操作仍可从系统托盘继续访问。",
    buttons: ["隐藏到托盘", "完全退出", "取消"],
    cancelId: 2,
    defaultId: 0,
    checkboxLabel: "将本次选择设为默认行为",
  });

  if (result.response === 2) {
    return;
  }

  if (result.checkboxChecked) {
    const nextBehavior = result.response === 0 ? "hide_to_tray" : "exit_application";
    await coordinator.saveSettings({
      ...snapshot.settings,
      closeBehavior: nextBehavior,
    } satisfies LauncherSettings);
  }

  if (result.response === 0) {
    mainWindow?.hide();
  } else {
    shouldQuit = true;
    app.quit();
  }
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
      shouldQuit = true;
      void app.quit();
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
  tray.setToolTip(`RayleaBot 启动器 · ${launcherCopy.statusSummary(snapshot.serviceState)}`);
  tray.setContextMenu(menu);
}

async function createMainWindow() {
  nativeTheme.themeSource = "dark";

  mainWindow = new BrowserWindow({
    width: 1380,
    height: 920,
    minWidth: 1120,
    minHeight: 760,
    title: "RayleaBot 启动器",
    frame: false,
    roundedCorners: true,
    backgroundColor: "#00000000",
    show: false,
    autoHideMenuBar: true,
    webPreferences: {
      preload: resolvePreloadPath(),
      contextIsolation: true,
      sandbox: false,
    },
  });

  mainWindow.on("ready-to-show", () => {
    if (process.platform === "win32") {
      mainWindow?.setBackgroundMaterial("acrylic");
    }
    mainWindow?.show();
  });

  mainWindow.on("maximize", () => {
    windowMaximized = true;
    mainWindow?.webContents.send("launcher:maximized-change", true);
  });

  mainWindow.on("unmaximize", () => {
    windowMaximized = false;
    mainWindow?.webContents.send("launcher:maximized-change", false);
  });

  mainWindow.on("close", (event) => {
    if (shouldQuit) {
      return;
    }
    event.preventDefault();
    void handleCloseRequest();
  });

  if (devServerUrl) {
    await mainWindow.loadURL(devServerUrl);
  } else {
    await mainWindow.loadFile(resolveRendererPath());
  }
}

function wireIpc() {
  ipcMain.handle("launcher:minimize", () => mainWindow?.minimize());
  ipcMain.handle("launcher:maximize", () => {
    if (!mainWindow) return;
    if (windowMaximized) {
      mainWindow.unmaximize();
    } else {
      mainWindow.maximize();
    }
  });
  ipcMain.handle("launcher:close", () => handleCloseRequest());
  ipcMain.handle("launcher:is-maximized", () => windowMaximized);
  ipcMain.handle("launcher:get-platform", async () => `${process.platform}-${process.arch}`);
  ipcMain.handle("launcher:get-snapshot", async () => coordinator.snapshot);
  ipcMain.handle("launcher:initialize", async () => coordinator.initialize());
  ipcMain.handle("launcher:refresh", async () => coordinator.refresh());
  ipcMain.handle("launcher:retry", async () => coordinator.retry());
  ipcMain.handle("launcher:start", async () => coordinator.start());
  ipcMain.handle("launcher:stop", async () => coordinator.stop());
  ipcMain.handle("launcher:open-web", async () => coordinator.openWebUi());
  ipcMain.handle("launcher:open-release-page", async () => coordinator.openReleasePage());
  ipcMain.handle("launcher:open-logs", async () => coordinator.openLogsDirectory());
  ipcMain.handle("launcher:save-settings", async (_event, settings: LauncherSettings) => coordinator.saveSettings(settings));
  ipcMain.handle("launcher:choose-server", async () => chooseServerExecutable());
  ipcMain.handle("launcher:choose-config", async () => chooseConfigFile());
  ipcMain.handle("launcher:choose-workdir", async () => chooseWorkdir());
  ipcMain.handle("launcher:exit", async () => {
    shouldQuit = true;
    app.quit();
  });
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
    mainWindow?.webContents.send("launcher:snapshot", snapshot);
  });

  await coordinator.initialize();
}

app.on("window-all-closed", () => {
  if (shouldQuit) {
    app.quit();
  }
});

app.on("before-quit", () => {
  shouldQuit = true;
});

void bootstrap();

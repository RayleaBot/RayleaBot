import { contextBridge, ipcRenderer } from "electron";
import type { LauncherDesktopApi } from "../shared/desktop-api";
import type { LauncherSettings, LauncherSnapshot } from "../shared/launcher-models";

const api: LauncherDesktopApi = {
  getPlatform: () => ipcRenderer.invoke("launcher:get-platform"),
  getSnapshot: () => ipcRenderer.invoke("launcher:get-snapshot"),
  initialize: () => ipcRenderer.invoke("launcher:initialize"),
  refresh: () => ipcRenderer.invoke("launcher:refresh"),
  retry: () => ipcRenderer.invoke("launcher:retry"),
  start: () => ipcRenderer.invoke("launcher:start"),
  stop: () => ipcRenderer.invoke("launcher:stop"),
  openWebUi: () => ipcRenderer.invoke("launcher:open-web"),
  openReleasePage: () => ipcRenderer.invoke("launcher:open-release-page"),
  openLogsDirectory: () => ipcRenderer.invoke("launcher:open-logs"),
  saveSettings: (settings: LauncherSettings) => ipcRenderer.invoke("launcher:save-settings", settings),
  chooseServerExecutable: () => ipcRenderer.invoke("launcher:choose-server"),
  chooseConfigFile: () => ipcRenderer.invoke("launcher:choose-config"),
  chooseWorkdir: () => ipcRenderer.invoke("launcher:choose-workdir"),
  exitApplication: () => ipcRenderer.invoke("launcher:exit"),
  minimize: () => ipcRenderer.invoke("launcher:minimize"),
  maximize: () => ipcRenderer.invoke("launcher:maximize"),
  close: () => ipcRenderer.invoke("launcher:close"),
  isMaximized: () => ipcRenderer.invoke("launcher:is-maximized"),
  onSnapshot(listener: (snapshot: LauncherSnapshot) => void) {
    const handler = (_event: unknown, snapshot: LauncherSnapshot) => listener(snapshot);
    ipcRenderer.on("launcher:snapshot", handler);
    return () => {
      ipcRenderer.off("launcher:snapshot", handler);
    };
  },
  onMaximizedChange(listener: (maximized: boolean) => void) {
    const handler = (_event: unknown, maximized: boolean) => listener(maximized);
    ipcRenderer.on("launcher:maximized-change", handler);
    return () => {
      ipcRenderer.off("launcher:maximized-change", handler);
    };
  },
};

contextBridge.exposeInMainWorld("rayleaLauncher", api);

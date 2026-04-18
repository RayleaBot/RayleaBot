import { contextBridge, ipcRenderer } from "electron";
import type { LauncherDesktopApi } from "../shared/desktop-api";
import type { LauncherSettings, LauncherSnapshot } from "../shared/launcher-models";
import { launcherEventChannels, launcherInvokeChannels } from "../shared/launcher-ipc";

const api: LauncherDesktopApi = {
  getPlatform: () => ipcRenderer.invoke(launcherInvokeChannels.getPlatform),
  getSnapshot: () => ipcRenderer.invoke(launcherInvokeChannels.getSnapshot),
  initialize: () => ipcRenderer.invoke(launcherInvokeChannels.initialize),
  refresh: () => ipcRenderer.invoke(launcherInvokeChannels.refresh),
  retry: () => ipcRenderer.invoke(launcherInvokeChannels.retry),
  start: () => ipcRenderer.invoke(launcherInvokeChannels.start),
  stop: () => ipcRenderer.invoke(launcherInvokeChannels.stop),
  resetAdmin: () => ipcRenderer.invoke(launcherInvokeChannels.resetAdmin),
  openWebUi: (targetPath?: string) => ipcRenderer.invoke(launcherInvokeChannels.openWeb, targetPath),
  createRecoveryRecheck: () => ipcRenderer.invoke(launcherInvokeChannels.createRecoveryRecheck),
  createRuntimeBootstrap: (resources?: string[]) => ipcRenderer.invoke(launcherInvokeChannels.createRuntimeBootstrap, resources),
  openReleasePage: () => ipcRenderer.invoke(launcherInvokeChannels.openReleasePage),
  openLogsDirectory: () => ipcRenderer.invoke(launcherInvokeChannels.openLogs),
  saveSettings: (settings: LauncherSettings) => ipcRenderer.invoke(launcherInvokeChannels.saveSettings, settings),
  previewResolvedSettings: (settings: LauncherSettings) => ipcRenderer.invoke(launcherInvokeChannels.previewResolvedSettings, settings),
  chooseInstallationRoot: () => ipcRenderer.invoke(launcherInvokeChannels.chooseInstallationRoot),
  chooseServerExecutable: () => ipcRenderer.invoke(launcherInvokeChannels.chooseServer),
  chooseConfigFile: () => ipcRenderer.invoke(launcherInvokeChannels.chooseConfig),
  chooseWorkdir: () => ipcRenderer.invoke(launcherInvokeChannels.chooseWorkdir),
  exitApplication: () => ipcRenderer.invoke(launcherInvokeChannels.exit),
  minimize: () => ipcRenderer.invoke(launcherInvokeChannels.minimize),
  maximize: () => ipcRenderer.invoke(launcherInvokeChannels.maximize),
  close: () => ipcRenderer.invoke(launcherInvokeChannels.close),
  closeConfirmResponse: (response) => ipcRenderer.invoke(launcherInvokeChannels.closeConfirmResponse, response),
  isMaximized: () => ipcRenderer.invoke(launcherInvokeChannels.isMaximized),
  onSnapshot(listener: (snapshot: LauncherSnapshot) => void) {
    const handler = (_event: unknown, snapshot: LauncherSnapshot) => listener(snapshot);
    ipcRenderer.on(launcherEventChannels.snapshot, handler);
    return () => {
      ipcRenderer.off(launcherEventChannels.snapshot, handler);
    };
  },
  onMaximizedChange(listener: (maximized: boolean) => void) {
    const handler = (_event: unknown, maximized: boolean) => listener(maximized);
    ipcRenderer.on(launcherEventChannels.maximizedChange, handler);
    return () => {
      ipcRenderer.off(launcherEventChannels.maximizedChange, handler);
    };
  },
  onShowExitConfirm(listener: () => void) {
    const handler = () => listener();
    ipcRenderer.on(launcherEventChannels.showExitConfirm, handler);
    return () => {
      ipcRenderer.off(launcherEventChannels.showExitConfirm, handler);
    };
  },
};

contextBridge.exposeInMainWorld("rayleaLauncher", api);

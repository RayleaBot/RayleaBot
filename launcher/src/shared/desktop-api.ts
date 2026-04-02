import type { LauncherSettings, LauncherSnapshot } from "./launcher-models";

export interface LauncherDesktopApi {
  getPlatform(): Promise<string>;
  getSnapshot(): Promise<LauncherSnapshot>;
  initialize(): Promise<void>;
  refresh(): Promise<void>;
  retry(): Promise<void>;
  start(): Promise<void>;
  stop(): Promise<void>;
  resetAdmin(): Promise<void>;
  openWebUi(targetPath?: string): Promise<void>;
  createRecoveryRecheck(): Promise<void>;
  createRuntimeBootstrap(resources?: string[]): Promise<void>;
  openReleasePage(): Promise<void>;
  openLogsDirectory(): Promise<void>;
  saveSettings(settings: LauncherSettings): Promise<void>;
  chooseInstallationRoot(): Promise<string | null>;
  chooseServerExecutable(): Promise<string | null>;
  chooseConfigFile(): Promise<string | null>;
  chooseWorkdir(): Promise<string | null>;
  exitApplication(): Promise<void>;
  minimize(): Promise<void>;
  maximize(): Promise<void>;
  close(): Promise<void>;
  isMaximized(): Promise<boolean>;
  onSnapshot(listener: (snapshot: LauncherSnapshot) => void): () => void;
  onMaximizedChange(listener: (maximized: boolean) => void): () => void;
}

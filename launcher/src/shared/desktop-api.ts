import type { LauncherSettings, LauncherSnapshot } from "./launcher-models";

export interface LauncherDesktopApi {
  getPlatform(): Promise<string>;
  getSnapshot(): Promise<LauncherSnapshot>;
  initialize(): Promise<void>;
  refresh(): Promise<void>;
  retry(): Promise<void>;
  start(): Promise<void>;
  stop(): Promise<void>;
  openWebUi(): Promise<void>;
  openReleasePage(): Promise<void>;
  openLogsDirectory(): Promise<void>;
  saveSettings(settings: LauncherSettings): Promise<void>;
  chooseServerExecutable(): Promise<string | null>;
  chooseConfigFile(): Promise<string | null>;
  chooseWorkdir(): Promise<string | null>;
  exitApplication(): Promise<void>;
  onSnapshot(listener: (snapshot: LauncherSnapshot) => void): () => void;
}

import fs from "node:fs/promises";
import path from "node:path";
import type { LauncherCloseBehavior, LauncherSettings } from "../../shared/launcher-models";

interface SerializedLauncherSettings {
  serverExecutablePath?: string;
  configPath?: string;
  workdir?: string;
  closeBehavior?: string;
  CloseToTrayEnabled?: boolean;
  ServerExecutablePath?: string;
  ConfigPath?: string;
  Workdir?: string;
  CloseBehavior?: string;
}

function normalizeCloseBehavior(value: unknown): LauncherCloseBehavior {
  if (value === "hide_to_tray" || value === "HideToTray") {
    return "hide_to_tray";
  }
  if (value === "exit_application" || value === "ExitApplication") {
    return "exit_application";
  }
  if (value === "ask_every_time" || value === "AskEveryTime") {
    return "ask_every_time";
  }
  if (typeof value === "boolean") {
    return value ? "hide_to_tray" : "ask_every_time";
  }
  return "ask_every_time";
}

function readString(...values: unknown[]) {
  for (const value of values) {
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }
  return "";
}

async function pathExists(targetPath: string) {
  try {
    await fs.access(targetPath);
    return true;
  } catch {
    return false;
  }
}

async function findWorkspaceRoot(startPath: string) {
  let current = path.resolve(startPath);
  while (true) {
    const hasContracts = await pathExists(path.join(current, "contracts", "config.user.schema.json"));
    const hasServer = await pathExists(path.join(current, "server", "go.mod"));
    const hasLauncher = await pathExists(path.join(current, "launcher", "package.json"));
    const hasReleaseConfig = await pathExists(path.join(current, "config", "default.yaml"));
    const hasDeps = await pathExists(path.join(current, ".deps", "manifest.json"));

    if ((hasContracts && hasServer && hasLauncher) || (hasReleaseConfig && hasDeps)) {
      return current;
    }

    const parent = path.dirname(current);
    if (parent === current) {
      return startPath;
    }
    current = parent;
  }
}

function rebaseWorkspacePath(savedPath: string, savedWorkdir: string, currentWorkdir: string) {
  if (!savedPath || !savedWorkdir) {
    return "";
  }

  const relativePath = path.relative(savedWorkdir, savedPath);
  if (!relativePath || relativePath.startsWith("..") || path.isAbsolute(relativePath)) {
    return "";
  }

  return path.join(currentWorkdir, relativePath);
}

async function resolveStoredPath(savedPath: string, savedWorkdir: string, defaults: LauncherSettings[keyof LauncherSettings], defaultWorkdir: string) {
  if (savedPath && (await pathExists(savedPath))) {
    return savedPath;
  }

  const rebasedPath = rebaseWorkspacePath(savedPath, savedWorkdir, defaultWorkdir);
  if (rebasedPath && (await pathExists(rebasedPath))) {
    return rebasedPath;
  }

  if (typeof defaults === "string" && defaults && (await pathExists(defaults))) {
    return defaults;
  }

  return savedPath || (typeof defaults === "string" ? defaults : "");
}

function serverExecutableName(platform: NodeJS.Platform) {
  return platform === "win32" ? "raylea-server.exe" : "raylea-server";
}

export async function createDefaultSettings(basePath: string, platform: NodeJS.Platform): Promise<LauncherSettings> {
  const root = await findWorkspaceRoot(basePath);
  const executable = serverExecutableName(platform);
  const candidates = [
    path.join(root, executable),
    path.join(root, "server", executable),
    path.join(root, "server", "bin", executable),
  ];
  const serverExecutablePath = (await Promise.all(candidates.map(async (candidate) => ((await pathExists(candidate)) ? candidate : ""))))
    .find(Boolean) || candidates[0];

  return {
    serverExecutablePath,
    configPath: path.join(root, "config", "user.yaml"),
    workdir: root,
    closeBehavior: "ask_every_time",
  };
}

export class JsonLauncherSettingsStore {
  private readonly settingsPath: string;
  private readonly defaultsPromise: Promise<LauncherSettings>;

  constructor(userDataPath: string, basePath: string, platform: NodeJS.Platform) {
    this.settingsPath = path.join(userDataPath, "launcher.json");
    this.defaultsPromise = createDefaultSettings(basePath, platform);
  }

  async load() {
    const defaults = await this.defaultsPromise;
    if (!(await pathExists(this.settingsPath))) {
      await this.save(defaults);
      return defaults;
    }

    const payload = JSON.parse(await fs.readFile(this.settingsPath, "utf8")) as SerializedLauncherSettings;
    const savedWorkdir = readString(payload.workdir, payload.Workdir);
    const savedServerExecutablePath = readString(payload.serverExecutablePath, payload.ServerExecutablePath);
    const savedConfigPath = readString(payload.configPath, payload.ConfigPath);

    const serverExecutablePath = await resolveStoredPath(savedServerExecutablePath, savedWorkdir, defaults.serverExecutablePath, defaults.workdir);
    const configPath = await resolveStoredPath(savedConfigPath, savedWorkdir, defaults.configPath, defaults.workdir);
    const keepSavedWorkdir =
      savedWorkdir &&
      (await pathExists(savedWorkdir)) &&
      (!savedServerExecutablePath || savedServerExecutablePath === serverExecutablePath) &&
      (!savedConfigPath || savedConfigPath === configPath);

    const resolvedSettings = {
      serverExecutablePath,
      configPath,
      workdir: keepSavedWorkdir ? savedWorkdir : defaults.workdir,
      closeBehavior: normalizeCloseBehavior(payload.closeBehavior ?? payload.CloseBehavior ?? payload.CloseToTrayEnabled),
    } satisfies LauncherSettings;

    const normalizedSavedSettings = {
      serverExecutablePath: savedServerExecutablePath || defaults.serverExecutablePath,
      configPath: savedConfigPath || defaults.configPath,
      workdir: savedWorkdir || defaults.workdir,
      closeBehavior: normalizeCloseBehavior(payload.closeBehavior ?? payload.CloseBehavior ?? payload.CloseToTrayEnabled),
    } satisfies LauncherSettings;

    if (JSON.stringify(normalizedSavedSettings) !== JSON.stringify(resolvedSettings)) {
      await this.save(resolvedSettings);
    }

    return resolvedSettings;
  }

  async save(settings: LauncherSettings) {
    await fs.mkdir(path.dirname(this.settingsPath), { recursive: true });
    await fs.writeFile(this.settingsPath, JSON.stringify(settings, null, 2), "utf8");
  }
}

import fs from "node:fs/promises";
import path from "node:path";
import type { LauncherCloseBehavior, LauncherSettings } from "../../shared/launcher-models";

interface SerializedLauncherSettings {
  serverExecutablePath?: string;
  configPath?: string;
  workdir?: string;
  closeBehavior?: string;
  CloseToTrayEnabled?: boolean;
}

function normalizeCloseBehavior(value: unknown): LauncherCloseBehavior {
  if (value === "hide_to_tray" || value === "exit_application" || value === "ask_every_time") {
    return value;
  }
  if (typeof value === "boolean") {
    return value ? "hide_to_tray" : "ask_every_time";
  }
  return "ask_every_time";
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
      return defaults;
    }

    const payload = JSON.parse(await fs.readFile(this.settingsPath, "utf8")) as SerializedLauncherSettings;
    return {
      serverExecutablePath: payload.serverExecutablePath || defaults.serverExecutablePath,
      configPath: payload.configPath || defaults.configPath,
      workdir: payload.workdir || defaults.workdir,
      closeBehavior: normalizeCloseBehavior(payload.closeBehavior ?? payload.CloseToTrayEnabled),
    } satisfies LauncherSettings;
  }

  async save(settings: LauncherSettings) {
    await fs.mkdir(path.dirname(this.settingsPath), { recursive: true });
    await fs.writeFile(this.settingsPath, JSON.stringify(settings, null, 2), "utf8");
  }
}

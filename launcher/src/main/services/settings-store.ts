import fs from "node:fs/promises";
import path from "node:path";
import type {
  LauncherAdvancedOverrides,
  LauncherCloseBehavior,
  LauncherResolvedSettings,
  LauncherSettings,
} from "../../shared/launcher-models";

interface SerializedLauncherSettings {
  installationRoot?: string;
  advancedOverrides?: unknown;
  closeBehavior?: string;
  serverExecutablePath?: string;
  configPath?: string;
  workdir?: string;
  CloseToTrayEnabled?: boolean;
  InstallationRoot?: string;
  AdvancedOverrides?: unknown;
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

function serverExecutableName(platform: NodeJS.Platform) {
  return platform === "win32" ? "raylea-server.exe" : "raylea-server";
}

async function pathExists(targetPath: string) {
  try {
    await fs.access(targetPath);
    return true;
  } catch {
    return false;
  }
}

async function hasInstallationMarkers(root: string) {
  const hasContracts = await pathExists(path.join(root, "contracts", "config.user.schema.json"));
  const hasServer = await pathExists(path.join(root, "server", "go.mod"));
  const hasLauncher = await pathExists(path.join(root, "launcher", "package.json"));
  const hasReleaseConfig = await pathExists(path.join(root, "config", "default.yaml"));
  const hasDeps = await pathExists(path.join(root, ".deps", "manifest.json"));

  return (hasContracts && hasServer && hasLauncher) || (hasReleaseConfig && hasDeps);
}

export async function findInstallationRoot(startPath: string) {
  let current = path.resolve(startPath);
  while (true) {
    if (await hasInstallationMarkers(current)) {
      return current;
    }

    const parent = path.dirname(current);
    if (parent === current) {
      return path.resolve(startPath);
    }
    current = parent;
  }
}

function rebaseWorkspacePath(savedPath: string, savedWorkdir: string, nextRoot: string) {
  if (!savedPath || !savedWorkdir) {
    return "";
  }

  const relativePath = path.relative(savedWorkdir, savedPath);
  if (!relativePath || relativePath.startsWith("..") || path.isAbsolute(relativePath)) {
    return "";
  }

  return path.join(nextRoot, relativePath);
}

async function resolveLegacyPath(savedPath: string, savedWorkdir: string, fallback: string, nextRoot: string) {
  if (savedPath && (await pathExists(savedPath))) {
    return path.resolve(savedPath);
  }

  const rebasedPath = rebaseWorkspacePath(savedPath, savedWorkdir, nextRoot);
  if (rebasedPath && (await pathExists(rebasedPath))) {
    return path.resolve(rebasedPath);
  }

  return path.resolve(fallback);
}

async function resolveLegacyWorkdir(savedWorkdir: string, fallback: string, nextRoot: string) {
  if (savedWorkdir && (await pathExists(savedWorkdir))) {
    const managedEntries = [
      "config",
      "data",
      "cache",
      "logs",
      "plugins",
      ".deps",
      "templates",
    ];
    const hasManagedState = (
      await Promise.all(managedEntries.map(async (entry) => pathExists(path.join(savedWorkdir, entry))))
    ).some(Boolean);
    if (hasManagedState) {
      return path.resolve(savedWorkdir);
    }
  }

  const rebasedPath = rebaseWorkspacePath(savedWorkdir, savedWorkdir, nextRoot);
  if (rebasedPath && (await pathExists(rebasedPath))) {
    return path.resolve(rebasedPath);
  }
  return path.resolve(fallback);
}

function normalizeOverrides(value: unknown): LauncherAdvancedOverrides | undefined {
  if (!value || typeof value !== "object") {
    return undefined;
  }

  const payload = value as Record<string, unknown>;
  const normalized = {
    serverExecutablePath: readString(payload.serverExecutablePath, payload.ServerExecutablePath),
    configPath: readString(payload.configPath, payload.ConfigPath),
    workdir: readString(payload.workdir, payload.Workdir),
  } satisfies LauncherAdvancedOverrides;

  if (!normalized.serverExecutablePath && !normalized.configPath && !normalized.workdir) {
    return undefined;
  }

  return normalized;
}

function normalizeInstallationRoot(value: string) {
  return path.resolve(value || ".");
}

function serverExecutableCandidates(root: string, platform: NodeJS.Platform) {
  const executable = serverExecutableName(platform);
  return [
    path.join(root, executable),
    path.join(root, "server", executable),
    path.join(root, "server", "bin", executable),
  ];
}

async function resolveServerExecutable(root: string, platform: NodeJS.Platform) {
  const candidates = serverExecutableCandidates(root, platform);
  const existing = (
    await Promise.all(
      candidates.map(async (candidate) => ((await pathExists(candidate)) ? candidate : "")),
    )
  ).find(Boolean);
  return existing || candidates[0];
}

export async function resolveLauncherSettings(
  settings: LauncherSettings,
  platform: NodeJS.Platform,
): Promise<LauncherResolvedSettings> {
  const installationRoot = normalizeInstallationRoot(settings.installationRoot);
  const overrides = settings.advancedOverrides;
  const derivedServer = await resolveServerExecutable(installationRoot, platform);
  const derivedConfig = path.join(installationRoot, "config", "user.yaml");
  const derivedWorkdir = installationRoot;

  return {
    installationRoot,
    serverExecutablePath: readString(overrides?.serverExecutablePath) || derivedServer,
    configPath: readString(overrides?.configPath) || derivedConfig,
    workdir: readString(overrides?.workdir) || derivedWorkdir,
  };
}

export async function createDefaultSettings(basePath: string): Promise<LauncherSettings> {
  return {
    installationRoot: await findInstallationRoot(basePath),
    closeBehavior: "ask_every_time",
  };
}

async function inferLegacyInstallationRoot(
  savedInstallationRoot: string,
  savedWorkdir: string,
  savedServerExecutablePath: string,
  savedConfigPath: string,
  fallbackRoot: string,
) {
  if (savedInstallationRoot) {
    return normalizeInstallationRoot(savedInstallationRoot);
  }

  const candidates = [
    savedWorkdir,
    savedServerExecutablePath ? path.dirname(savedServerExecutablePath) : "",
    savedConfigPath ? path.dirname(savedConfigPath) : "",
  ].filter(Boolean);

  for (const candidate of candidates) {
    const resolvedCandidate = path.resolve(candidate);
    if (await pathExists(resolvedCandidate)) {
      const inferredRoot = await findInstallationRoot(resolvedCandidate);
      if (await hasInstallationMarkers(inferredRoot)) {
        return inferredRoot;
      }
    }
    const parent = path.dirname(resolvedCandidate);
    if (parent !== resolvedCandidate && (await pathExists(parent))) {
      const inferredRoot = await findInstallationRoot(parent);
      if (await hasInstallationMarkers(inferredRoot)) {
        return inferredRoot;
      }
    }
  }

  return fallbackRoot;
}

async function normalizeSettings(
  payload: SerializedLauncherSettings,
  defaults: LauncherSettings,
  platform: NodeJS.Platform,
): Promise<LauncherSettings> {
  const savedInstallationRoot = readString(payload.installationRoot, payload.InstallationRoot);
  const savedWorkdir = readString(payload.workdir, payload.Workdir);
  const savedServerExecutablePath = readString(payload.serverExecutablePath, payload.ServerExecutablePath);
  const savedConfigPath = readString(payload.configPath, payload.ConfigPath);
  const explicitOverrides = normalizeOverrides(payload.advancedOverrides ?? payload.AdvancedOverrides);

  const installationRoot = await inferLegacyInstallationRoot(
    savedInstallationRoot,
    savedWorkdir,
    savedServerExecutablePath,
    savedConfigPath,
    defaults.installationRoot,
  );
  const baseSettings = {
    installationRoot,
    closeBehavior: normalizeCloseBehavior(payload.closeBehavior ?? payload.CloseBehavior ?? payload.CloseToTrayEnabled),
  } satisfies LauncherSettings;
  const derived = await resolveLauncherSettings(baseSettings, platform);

  const mergedOverrides = {
    serverExecutablePath:
      explicitOverrides?.serverExecutablePath
      || (savedServerExecutablePath
        ? await resolveLegacyPath(savedServerExecutablePath, savedWorkdir, derived.serverExecutablePath, installationRoot)
        : ""),
    configPath:
      explicitOverrides?.configPath
      || (savedConfigPath
        ? await resolveLegacyPath(savedConfigPath, savedWorkdir, derived.configPath, installationRoot)
        : ""),
    workdir:
      explicitOverrides?.workdir
      || (savedWorkdir
        ? await resolveLegacyWorkdir(savedWorkdir, derived.workdir, installationRoot)
        : ""),
  } satisfies LauncherAdvancedOverrides;

  const normalizedOverrides = {
    serverExecutablePath:
      mergedOverrides.serverExecutablePath && path.resolve(mergedOverrides.serverExecutablePath) !== derived.serverExecutablePath
        ? path.resolve(mergedOverrides.serverExecutablePath)
        : undefined,
    configPath:
      mergedOverrides.configPath && path.resolve(mergedOverrides.configPath) !== derived.configPath
        ? path.resolve(mergedOverrides.configPath)
        : undefined,
    workdir:
      mergedOverrides.workdir && path.resolve(mergedOverrides.workdir) !== derived.workdir
        ? path.resolve(mergedOverrides.workdir)
        : undefined,
  } satisfies LauncherAdvancedOverrides;

  return {
    ...baseSettings,
    advancedOverrides: normalizeOverrides(normalizedOverrides),
  };
}

export class JsonLauncherSettingsStore {
  private readonly settingsPath: string;
  private readonly defaultsPromise: Promise<LauncherSettings>;
  private readonly platform: NodeJS.Platform;

  constructor(userDataPath: string, basePath: string, platform: NodeJS.Platform) {
    this.settingsPath = path.join(userDataPath, "launcher.json");
    this.defaultsPromise = createDefaultSettings(basePath);
    this.platform = platform;
  }

  async load() {
    const defaults = await this.defaultsPromise;
    if (!(await pathExists(this.settingsPath))) {
      await this.save(defaults);
      return defaults;
    }

    const payload = JSON.parse(await fs.readFile(this.settingsPath, "utf8")) as SerializedLauncherSettings;
    const normalized = await normalizeSettings(payload, defaults, this.platform);
    if (JSON.stringify(payload) !== JSON.stringify(normalized)) {
      await this.save(normalized);
    }
    return normalized;
  }

  async save(settings: LauncherSettings) {
    const normalized = await normalizeSettings(
      {
        installationRoot: settings.installationRoot,
        advancedOverrides: settings.advancedOverrides,
        closeBehavior: settings.closeBehavior,
      },
      await this.defaultsPromise,
      this.platform,
    );
    await fs.mkdir(path.dirname(this.settingsPath), { recursive: true });
    await fs.writeFile(this.settingsPath, JSON.stringify(normalized, null, 2), "utf8");
  }
}

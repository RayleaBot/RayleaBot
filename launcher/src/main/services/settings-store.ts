import fs from "node:fs/promises";
import path from "node:path";
import type {
  LauncherAdvancedOverrides,
  LauncherCloseBehavior,
  LauncherResolvedSettings,
  LauncherSettings,
} from "../../shared/launcher-models";
import { pathExists } from "./fs-utils";

interface SerializedLauncherSettings {
  installationRoot?: string;
  advancedOverrides?: unknown;
  closeBehavior?: string;
}

interface LauncherSettingsFileLocations {
  settingsPath: string;
}

interface ReadSerializedSettingsResult {
  path: string;
  payload: SerializedLauncherSettings | null;
  exists: boolean;
  valid: boolean;
}

function normalizeCloseBehavior(value: unknown): LauncherCloseBehavior {
  if (value === undefined || value === null || value === "") {
    return "ask_every_time";
  }
  if (value === "hide_to_tray") {
    return "hide_to_tray";
  }
  if (value === "exit_application") {
    return "exit_application";
  }
  if (value === "ask_every_time") {
    return "ask_every_time";
  }
  throw new Error("launcher closeBehavior must use a supported value.");
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

async function hasInstallationMarkers(root: string) {
  const hasServer = await pathExists(path.join(root, "server", "go.mod"));
  const hasLauncher = await pathExists(path.join(root, "launcher", "package.json"));
  const hasReleaseConfig = await pathExists(path.join(root, "config", "default.yaml"));
  const hasDeps = await pathExists(path.join(root, ".deps", "manifest.json"));

  return (hasServer && hasLauncher) || (hasReleaseConfig && hasDeps);
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

function normalizeOverrides(value: unknown): LauncherAdvancedOverrides | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return undefined;
  }

  const payload = value as Record<string, unknown>;
  const normalized = {
    serverExecutablePath: readString(payload.serverExecutablePath),
    configPath: readString(payload.configPath),
    workdir: readString(payload.workdir),
  } satisfies LauncherAdvancedOverrides;

  if (!normalized.serverExecutablePath && !normalized.configPath && !normalized.workdir) {
    return undefined;
  }

  return normalized;
}

function normalizeInstallationRoot(value: string) {
  return path.resolve(value || ".");
}

function findManagedWorktreeOwnerRoot(targetPath: string) {
  let current = path.resolve(targetPath);
  while (true) {
    const parent = path.dirname(current);
    if (parent === current) {
      return "";
    }
    if (path.basename(parent) === ".worktrees") {
      return path.dirname(parent);
    }
    current = parent;
  }
}

function resolveLauncherStorageRoot(basePath: string) {
  const ownerRoot = findManagedWorktreeOwnerRoot(basePath);
  if (ownerRoot) {
    return ownerRoot;
  }
  return path.resolve(basePath);
}

function resolveLauncherSettingsFileLocations(
  installationRoot: string,
): LauncherSettingsFileLocations {
  const storageRoot = resolveLauncherStorageRoot(installationRoot);
  return {
    settingsPath: path.join(storageRoot, "data", "launcher.json"),
  };
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

async function normalizeSettings(
  payload: SerializedLauncherSettings,
  defaults: LauncherSettings,
  platform: NodeJS.Platform,
): Promise<LauncherSettings> {
  const savedInstallationRoot = readString(payload.installationRoot);
  const explicitOverrides = normalizeOverrides(payload.advancedOverrides);
  const baseSettings = {
    installationRoot: savedInstallationRoot
      ? normalizeInstallationRoot(savedInstallationRoot)
      : defaults.installationRoot,
    closeBehavior: normalizeCloseBehavior(payload.closeBehavior),
  } satisfies LauncherSettings;
  const derived = await resolveLauncherSettings(baseSettings, platform);

  const normalizedOverrides = {
    serverExecutablePath:
      explicitOverrides?.serverExecutablePath
      && path.resolve(explicitOverrides.serverExecutablePath) !== derived.serverExecutablePath
        ? path.resolve(explicitOverrides.serverExecutablePath)
        : undefined,
    configPath:
      explicitOverrides?.configPath
      && path.resolve(explicitOverrides.configPath) !== derived.configPath
        ? path.resolve(explicitOverrides.configPath)
        : undefined,
    workdir:
      explicitOverrides?.workdir
      && path.resolve(explicitOverrides.workdir) !== derived.workdir
        ? path.resolve(explicitOverrides.workdir)
        : undefined,
  } satisfies LauncherAdvancedOverrides;

  return {
    ...baseSettings,
    advancedOverrides: normalizeOverrides(normalizedOverrides),
  };
}

async function normalizeCurrentSettingsInput(
  settings: LauncherSettings,
  platform: NodeJS.Platform,
): Promise<LauncherSettings> {
  const installationRoot = normalizeInstallationRoot(settings.installationRoot);
  const baseSettings = {
    installationRoot,
    closeBehavior: normalizeCloseBehavior(settings.closeBehavior),
  } satisfies LauncherSettings;
  const explicitOverrides = normalizeOverrides(settings.advancedOverrides);
  const derived = await resolveLauncherSettings(baseSettings, platform);

  const normalizedOverrides = {
    serverExecutablePath:
      explicitOverrides?.serverExecutablePath
      && path.resolve(explicitOverrides.serverExecutablePath) !== derived.serverExecutablePath
        ? path.resolve(explicitOverrides.serverExecutablePath)
        : undefined,
    configPath:
      explicitOverrides?.configPath
      && path.resolve(explicitOverrides.configPath) !== derived.configPath
        ? path.resolve(explicitOverrides.configPath)
        : undefined,
    workdir:
      explicitOverrides?.workdir
      && path.resolve(explicitOverrides.workdir) !== derived.workdir
        ? path.resolve(explicitOverrides.workdir)
        : undefined,
  } satisfies LauncherAdvancedOverrides;

  return {
    ...baseSettings,
    advancedOverrides: normalizeOverrides(normalizedOverrides),
  };
}

async function serializeSettings(settings: LauncherSettings) {
  return {
    installationRoot: settings.installationRoot,
    advancedOverrides: settings.advancedOverrides,
    closeBehavior: settings.closeBehavior,
  } satisfies SerializedLauncherSettings;
}

async function readSerializedSettings(settingsPath: string): Promise<ReadSerializedSettingsResult> {
  if (!settingsPath || !(await pathExists(settingsPath))) {
    return {
      path: settingsPath,
      payload: null,
      exists: false,
      valid: false,
    };
  }

  try {
    const rawPayload = JSON.parse(await fs.readFile(settingsPath, "utf8"));
    if (!rawPayload || typeof rawPayload !== "object" || Array.isArray(rawPayload)) {
      throw new Error("launcher.json must contain an object payload.");
    }
    return {
      path: settingsPath,
      payload: rawPayload as SerializedLauncherSettings,
      exists: true,
      valid: true,
    };
  } catch {
    return {
      path: settingsPath,
      payload: null,
      exists: true,
      valid: false,
    };
  }
}

export class JsonLauncherSettingsStore {
  private readonly defaultsPromise: Promise<LauncherSettings>;
  private readonly fileLocationsPromise: Promise<LauncherSettingsFileLocations>;
  private readonly platform: NodeJS.Platform;

  constructor(basePath: string, platform: NodeJS.Platform) {
    this.defaultsPromise = createDefaultSettings(basePath);
    this.fileLocationsPromise = this.defaultsPromise.then((defaults) =>
      resolveLauncherSettingsFileLocations(defaults.installationRoot),
    );
    this.platform = platform;
  }

  async load() {
    const defaults = await this.defaultsPromise;
    const locations = await this.fileLocationsPromise;
    const current = await readSerializedSettings(locations.settingsPath);
    if (current.exists && !current.valid) {
      await this.save(defaults);
      return defaults;
    }

    if (!current.exists || !current.payload) {
      await this.save(defaults);
      return defaults;
    }

    const normalized = await normalizeSettings(current.payload, defaults, this.platform);
    const serialized = await serializeSettings(normalized);
    if (JSON.stringify(current.payload) !== JSON.stringify(serialized)) {
      await fs.mkdir(path.dirname(locations.settingsPath), { recursive: true });
      await fs.writeFile(locations.settingsPath, JSON.stringify(serialized, null, 2), "utf8");
    }
    return normalized;
  }

  async save(settings: LauncherSettings) {
    const locations = await this.fileLocationsPromise;
    const normalized = await normalizeCurrentSettingsInput(settings, this.platform);
    const serialized = await serializeSettings(normalized);
    await fs.mkdir(path.dirname(locations.settingsPath), { recursive: true });
    await fs.writeFile(locations.settingsPath, JSON.stringify(serialized, null, 2), "utf8");
  }
}

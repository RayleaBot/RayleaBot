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
  installationRootPinned?: boolean;
  launcherBasePath?: string;
  advancedOverrides?: unknown;
  closeBehavior?: string;
  serverExecutablePath?: string;
  configPath?: string;
  workdir?: string;
  CloseToTrayEnabled?: boolean;
  InstallationRoot?: string;
  InstallationRootPinned?: boolean;
  LauncherBasePath?: string;
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

function readBoolean(...values: unknown[]) {
  for (const value of values) {
    if (typeof value === "boolean") {
      return value;
    }
  }
  return undefined;
}

function serverExecutableName(platform: NodeJS.Platform) {
  return platform === "win32" ? "raylea-server.exe" : "raylea-server";
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
  if (!value || typeof value !== "object" || Array.isArray(value)) {
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

function pathsEqual(left: string, right: string) {
  const normalize = (value: string) => {
    const resolved = path.resolve(value);
    return process.platform === "win32" ? resolved.toLowerCase() : resolved;
  };
  return normalize(left) === normalize(right);
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

function belongsToSameManagedWorktreeFamily(left: string, right: string) {
  const leftResolved = path.resolve(left);
  const rightResolved = path.resolve(right);

  if (pathsEqual(leftResolved, rightResolved)) {
    return false;
  }

  const leftOwner = findManagedWorktreeOwnerRoot(leftResolved);
  const rightOwner = findManagedWorktreeOwnerRoot(rightResolved);

  if (leftOwner && rightOwner) {
    return pathsEqual(leftOwner, rightOwner);
  }
  if (leftOwner) {
    return pathsEqual(leftOwner, rightResolved);
  }
  if (rightOwner) {
    return pathsEqual(rightOwner, leftResolved);
  }
  return false;
}

function shouldUseCurrentInstallationRoot(
  savedInstallationRoot: string,
  savedLauncherBasePath: string,
  savedInstallationRootPinned: boolean | undefined,
  fallbackRoot: string,
  allowLegacyWorktreeMigration: boolean,
) {
  if (savedInstallationRootPinned === true) {
    return false;
  }

  if (savedInstallationRootPinned === false) {
    return !pathsEqual(savedInstallationRoot, fallbackRoot);
  }

  if (!allowLegacyWorktreeMigration) {
    return false;
  }

  if (savedLauncherBasePath && !pathsEqual(savedInstallationRoot, savedLauncherBasePath)) {
    return false;
  }

  return belongsToSameManagedWorktreeFamily(savedInstallationRoot, fallbackRoot);
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
  allowLegacyWorktreeMigration: boolean,
  savedLauncherBasePath: string,
  savedInstallationRootPinned: boolean | undefined,
) {
  if (savedInstallationRoot) {
    const normalizedSavedRoot = normalizeInstallationRoot(savedInstallationRoot);
    const normalizedSavedLauncherBasePath = savedLauncherBasePath
      ? normalizeInstallationRoot(savedLauncherBasePath)
      : "";
    if (
      shouldUseCurrentInstallationRoot(
        normalizedSavedRoot,
        normalizedSavedLauncherBasePath,
        savedInstallationRootPinned,
        fallbackRoot,
        allowLegacyWorktreeMigration,
      )
    ) {
      return fallbackRoot;
    }
    return normalizedSavedRoot;
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
  const savedInstallationRootPinned = readBoolean(payload.installationRootPinned, payload.InstallationRootPinned);
  const savedLauncherBasePath = readString(payload.launcherBasePath, payload.LauncherBasePath);
  const savedWorkdir = readString(payload.workdir, payload.Workdir);
  const savedServerExecutablePath = readString(payload.serverExecutablePath, payload.ServerExecutablePath);
  const savedConfigPath = readString(payload.configPath, payload.ConfigPath);
  const explicitOverrides = normalizeOverrides(payload.advancedOverrides ?? payload.AdvancedOverrides);
  const hasExplicitPathOverrides =
    Boolean(savedWorkdir)
    || Boolean(savedServerExecutablePath)
    || Boolean(savedConfigPath)
    || Boolean(explicitOverrides?.serverExecutablePath)
    || Boolean(explicitOverrides?.configPath)
    || Boolean(explicitOverrides?.workdir);

  const installationRoot = await inferLegacyInstallationRoot(
    savedInstallationRoot,
    savedWorkdir,
    savedServerExecutablePath,
    savedConfigPath,
    defaults.installationRoot,
    !hasExplicitPathOverrides,
    savedLauncherBasePath,
    savedInstallationRootPinned,
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

async function serializeSettings(
  settings: LauncherSettings,
  defaults: LauncherSettings,
) {
  return {
    installationRoot: settings.installationRoot,
    installationRootPinned: !pathsEqual(settings.installationRoot, defaults.installationRoot),
    launcherBasePath: defaults.installationRoot,
    advancedOverrides: settings.advancedOverrides,
    closeBehavior: settings.closeBehavior,
  } satisfies SerializedLauncherSettings;
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

    let payload: SerializedLauncherSettings;
    try {
      const rawPayload = JSON.parse(await fs.readFile(this.settingsPath, "utf8"));
      if (!rawPayload || typeof rawPayload !== "object" || Array.isArray(rawPayload)) {
        throw new Error("launcher.json must contain an object payload.");
      }
      payload = rawPayload as SerializedLauncherSettings;
    } catch {
      await this.save(defaults);
      return defaults;
    }
    const normalized = await normalizeSettings(payload, defaults, this.platform);
    const serialized = await serializeSettings(normalized, defaults);
    if (JSON.stringify(payload) !== JSON.stringify(serialized)) {
      await fs.mkdir(path.dirname(this.settingsPath), { recursive: true });
      await fs.writeFile(this.settingsPath, JSON.stringify(serialized, null, 2), "utf8");
    }
    return normalized;
  }

  async save(settings: LauncherSettings) {
    const defaults = await this.defaultsPromise;
    const normalized = await normalizeCurrentSettingsInput(settings, this.platform);
    const serialized = await serializeSettings(normalized, defaults);
    await fs.mkdir(path.dirname(this.settingsPath), { recursive: true });
    await fs.writeFile(this.settingsPath, JSON.stringify(serialized, null, 2), "utf8");
  }
}

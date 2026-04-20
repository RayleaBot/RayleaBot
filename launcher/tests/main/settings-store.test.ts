import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, test } from "vitest";
import {
  JsonLauncherSettingsStore,
  resolveLauncherSettings,
} from "@main/services/settings-store";

const tempRoots: string[] = [];

async function createWorkspace(root: string) {
  await fs.mkdir(path.join(root, "contracts"), { recursive: true });
  await fs.mkdir(path.join(root, "server"), { recursive: true });
  await fs.mkdir(path.join(root, "launcher"), { recursive: true });
  await fs.mkdir(path.join(root, "config"), { recursive: true });
  await fs.writeFile(path.join(root, "contracts", "config.user.schema.json"), "{}", "utf8");
  await fs.writeFile(path.join(root, "server", "go.mod"), "module raylea\n", "utf8");
  await fs.writeFile(path.join(root, "launcher", "package.json"), "{}", "utf8");
  await fs.writeFile(path.join(root, "config", "default.yaml"), "server: {}\n", "utf8");
  await fs.writeFile(path.join(root, "config", "user.yaml"), "server: {}\n", "utf8");
  await fs.writeFile(path.join(root, "server", "raylea-server.exe"), "binary", "utf8");
}

async function createTempDir(label: string) {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), `raylea-${label}-`));
  tempRoots.push(tempRoot);
  return tempRoot;
}

function launcherBasePath(root: string) {
  return path.join(root, "launcher");
}

function settingsPath(root: string) {
  return path.join(root, "data", "launcher.json");
}

function legacySettingsPath(userDataPath: string) {
  return path.join(userDataPath, "launcher.json");
}

async function fileExists(targetPath: string) {
  try {
    await fs.access(targetPath);
    return true;
  } catch {
    return false;
  }
}

async function readSettingsFile(targetPath: string) {
  return JSON.parse(await fs.readFile(targetPath, "utf8")) as {
    installationRoot?: string;
    installationRootPinned?: boolean;
    advancedOverrides?: {
      workdir?: string;
      configPath?: string;
      serverExecutablePath?: string;
    };
    closeBehavior?: string;
  };
}

afterEach(async () => {
  await Promise.all(
    tempRoots.splice(0).map(async (target) => {
      await fs.rm(target, { recursive: true, force: true });
    }),
  );
});

describe("launcher settings store", () => {
  test("persists installation-root defaults to data/launcher.json when the settings file is missing", async () => {
    const currentRoot = await createTempDir("default-workspace");
    const userDataPath = await createTempDir("default-userdata");

    await createWorkspace(currentRoot);

    const store = new JsonLauncherSettingsStore(launcherBasePath(currentRoot), "win32", userDataPath);
    const loaded = await store.load();
    const resolved = await resolveLauncherSettings(loaded, "win32");

    expect(loaded.installationRoot).toBe(currentRoot);
    expect(loaded.advancedOverrides).toBeUndefined();
    expect(resolved.serverExecutablePath).toBe(path.join(currentRoot, "server", "raylea-server.exe"));
    expect(resolved.configPath).toBe(path.join(currentRoot, "config", "user.yaml"));
    expect(resolved.workdir).toBe(currentRoot);

    const saved = await readSettingsFile(settingsPath(currentRoot));
    expect(saved.installationRoot).toBe(currentRoot);
    expect(saved.advancedOverrides).toBeUndefined();
    expect(await fileExists(legacySettingsPath(userDataPath))).toBe(false);
  });

  test("recovers to defaults when data/launcher.json is malformed", async () => {
    const currentRoot = await createTempDir("corrupt-workspace");
    const userDataPath = await createTempDir("corrupt-userdata");

    await createWorkspace(currentRoot);
    await fs.mkdir(path.join(currentRoot, "data"), { recursive: true });
    await fs.writeFile(settingsPath(currentRoot), "{not valid json", "utf8");

    const store = new JsonLauncherSettingsStore(launcherBasePath(currentRoot), "win32", userDataPath);
    const loaded = await store.load();
    const resolved = await resolveLauncherSettings(loaded, "win32");

    expect(loaded.installationRoot).toBe(currentRoot);
    expect(loaded.closeBehavior).toBe("ask_every_time");
    expect(loaded.advancedOverrides).toBeUndefined();
    expect(resolved.serverExecutablePath).toBe(path.join(currentRoot, "server", "raylea-server.exe"));
    expect(resolved.configPath).toBe(path.join(currentRoot, "config", "user.yaml"));
    expect(resolved.workdir).toBe(currentRoot);

    const persisted = await readSettingsFile(settingsPath(currentRoot));
    expect(persisted.installationRoot).toBe(currentRoot);
    expect(persisted.closeBehavior).toBe("ask_every_time");
  });

  test("migrates legacy userData settings into data/launcher.json", async () => {
    const previousRoot = await createTempDir("old-workspace");
    const currentRoot = await createTempDir("current-workspace");
    const userDataPath = await createTempDir("userdata");

    await createWorkspace(currentRoot);
    await fs.writeFile(
      legacySettingsPath(userDataPath),
      JSON.stringify(
        {
          serverExecutablePath: path.join(previousRoot, "server", "raylea-server.exe"),
          configPath: path.join(previousRoot, "config", "user.yaml"),
          workdir: previousRoot,
          closeBehavior: "hide_to_tray",
        },
        null,
        2,
      ),
      "utf8",
    );

    const store = new JsonLauncherSettingsStore(launcherBasePath(currentRoot), "win32", userDataPath);
    const loaded = await store.load();
    const resolved = await resolveLauncherSettings(loaded, "win32");

    expect(loaded.installationRoot).toBe(currentRoot);
    expect(loaded.closeBehavior).toBe("hide_to_tray");
    expect(loaded.advancedOverrides).toBeUndefined();
    expect(resolved.serverExecutablePath).toBe(path.join(currentRoot, "server", "raylea-server.exe"));
    expect(resolved.configPath).toBe(path.join(currentRoot, "config", "user.yaml"));
    expect(resolved.workdir).toBe(currentRoot);

    const migrated = await readSettingsFile(settingsPath(currentRoot));
    expect(migrated.installationRoot).toBe(currentRoot);
    expect(migrated.closeBehavior).toBe("hide_to_tray");
    expect(await fileExists(legacySettingsPath(userDataPath))).toBe(true);
  });

  test("prefers data/launcher.json when current and legacy settings both exist", async () => {
    const currentRoot = await createTempDir("priority-workspace");
    const userDataPath = await createTempDir("priority-userdata");

    await createWorkspace(currentRoot);
    await fs.mkdir(path.join(currentRoot, "data"), { recursive: true });
    await fs.writeFile(
      settingsPath(currentRoot),
      JSON.stringify(
        {
          installationRoot: currentRoot,
          closeBehavior: "hide_to_tray",
        },
        null,
        2,
      ),
      "utf8",
    );
    await fs.writeFile(
      legacySettingsPath(userDataPath),
      JSON.stringify(
        {
          installationRoot: currentRoot,
          closeBehavior: "exit_application",
        },
        null,
        2,
      ),
      "utf8",
    );

    const store = new JsonLauncherSettingsStore(launcherBasePath(currentRoot), "win32", userDataPath);
    const loaded = await store.load();

    expect(loaded.closeBehavior).toBe("hide_to_tray");

    const saved = await readSettingsFile(settingsPath(currentRoot));
    expect(saved.closeBehavior).toBe("hide_to_tray");
  });

  test("accepts legacy PascalCase settings keys and normalizes them to installation-root settings", async () => {
    const currentRoot = await createTempDir("legacy-workspace");
    const userDataPath = await createTempDir("legacy-userdata");

    await createWorkspace(currentRoot);
    await fs.writeFile(
      legacySettingsPath(userDataPath),
      JSON.stringify(
        {
          ServerExecutablePath: path.join(currentRoot, "server", "raylea-server.exe"),
          ConfigPath: path.join(currentRoot, "config", "user.yaml"),
          Workdir: currentRoot,
          CloseBehavior: "hide_to_tray",
        },
        null,
        2,
      ),
      "utf8",
    );

    const store = new JsonLauncherSettingsStore(launcherBasePath(currentRoot), "win32", userDataPath);
    const loaded = await store.load();
    const resolved = await resolveLauncherSettings(loaded, "win32");

    expect(loaded.installationRoot).toBe(currentRoot);
    expect(loaded.closeBehavior).toBe("hide_to_tray");
    expect(resolved.serverExecutablePath).toBe(path.join(currentRoot, "server", "raylea-server.exe"));
    expect(resolved.configPath).toBe(path.join(currentRoot, "config", "user.yaml"));
    expect(resolved.workdir).toBe(currentRoot);

    const migrated = await readSettingsFile(settingsPath(currentRoot));
    expect(migrated.closeBehavior).toBe("hide_to_tray");
  });

  test("keeps explicit advanced overrides when they differ from installation-root defaults", async () => {
    const currentRoot = await createTempDir("override-workspace");
    const userDataPath = await createTempDir("override-userdata");
    const altWorkdir = await createTempDir("override-workdir");

    await createWorkspace(currentRoot);

    const store = new JsonLauncherSettingsStore(launcherBasePath(currentRoot), "win32", userDataPath);
    await store.save({
      installationRoot: currentRoot,
      closeBehavior: "ask_every_time",
      advancedOverrides: {
        workdir: altWorkdir,
      },
    });

    const loaded = await store.load();
    const resolved = await resolveLauncherSettings(loaded, "win32");

    expect(loaded.advancedOverrides?.workdir).toBe(altWorkdir);
    expect(resolved.workdir).toBe(altWorkdir);
    expect(resolved.serverExecutablePath).toBe(path.join(currentRoot, "server", "raylea-server.exe"));

    const persisted = await readSettingsFile(settingsPath(currentRoot));
    expect(persisted.advancedOverrides?.workdir).toBe(altWorkdir);
    expect(await fileExists(legacySettingsPath(userDataPath))).toBe(false);
  });

  test("stores worktree settings in the owner root data directory", async () => {
    const mainRoot = await createTempDir("shared-main-workspace");
    const worktreeRoot = path.join(mainRoot, ".worktrees", "web-cn-redesign");
    const userDataPath = await createTempDir("shared-userdata");

    await createWorkspace(mainRoot);
    await createWorkspace(worktreeRoot);

    const store = new JsonLauncherSettingsStore(launcherBasePath(worktreeRoot), "win32", userDataPath);
    const loaded = await store.load();

    expect(loaded.installationRoot).toBe(worktreeRoot);
    expect(await fileExists(settingsPath(mainRoot))).toBe(true);
    expect(await fileExists(settingsPath(worktreeRoot))).toBe(false);
  });

  test("prefers the current worktree root when saved defaults point at the main checkout", async () => {
    const mainRoot = await createTempDir("main-workspace");
    const worktreeRoot = path.join(mainRoot, ".worktrees", "web-cn-redesign");
    const userDataPath = await createTempDir("worktree-userdata");

    await createWorkspace(mainRoot);
    await createWorkspace(worktreeRoot);
    await fs.mkdir(path.join(mainRoot, "data"), { recursive: true });
    await fs.writeFile(
      settingsPath(mainRoot),
      JSON.stringify(
        {
          installationRoot: mainRoot,
          closeBehavior: "ask_every_time",
        },
        null,
        2,
      ),
      "utf8",
    );

    const store = new JsonLauncherSettingsStore(launcherBasePath(worktreeRoot), "win32", userDataPath);
    const loaded = await store.load();
    const resolved = await resolveLauncherSettings(loaded, "win32");

    expect(loaded.installationRoot).toBe(worktreeRoot);
    expect(loaded.advancedOverrides).toBeUndefined();
    expect(resolved.serverExecutablePath).toBe(path.join(worktreeRoot, "server", "raylea-server.exe"));
    expect(resolved.configPath).toBe(path.join(worktreeRoot, "config", "user.yaml"));
    expect(resolved.workdir).toBe(worktreeRoot);

    const persisted = await readSettingsFile(settingsPath(mainRoot));
    expect(persisted.installationRoot).toBe(worktreeRoot);
    expect(persisted.installationRootPinned).toBe(false);
  });

  test("preserves a manually pinned installation root across worktree launches", async () => {
    const mainRoot = await createTempDir("pinned-main-workspace");
    const worktreeRoot = path.join(mainRoot, ".worktrees", "web-cn-redesign");
    const userDataPath = await createTempDir("pinned-userdata");

    await createWorkspace(mainRoot);
    await createWorkspace(worktreeRoot);

    const store = new JsonLauncherSettingsStore(launcherBasePath(worktreeRoot), "win32", userDataPath);
    await store.save({
      installationRoot: mainRoot,
      closeBehavior: "ask_every_time",
    });

    const loaded = await store.load();
    const resolved = await resolveLauncherSettings(loaded, "win32");

    expect(loaded.installationRoot).toBe(mainRoot);
    expect(resolved.serverExecutablePath).toBe(path.join(mainRoot, "server", "raylea-server.exe"));
    expect(resolved.configPath).toBe(path.join(mainRoot, "config", "user.yaml"));
    expect(resolved.workdir).toBe(mainRoot);

    const persisted = await readSettingsFile(settingsPath(mainRoot));
    expect(persisted.installationRoot).toBe(mainRoot);
    expect(persisted.installationRootPinned).toBe(true);
  });
});

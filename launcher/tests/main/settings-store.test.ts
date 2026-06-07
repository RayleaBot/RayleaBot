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

    await createWorkspace(currentRoot);

    const store = new JsonLauncherSettingsStore(launcherBasePath(currentRoot), "win32");
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
  });

  test("recovers to defaults when data/launcher.json is malformed", async () => {
    const currentRoot = await createTempDir("corrupt-workspace");

    await createWorkspace(currentRoot);
    await fs.mkdir(path.join(currentRoot, "data"), { recursive: true });
    await fs.writeFile(settingsPath(currentRoot), "{not valid json", "utf8");

    const store = new JsonLauncherSettingsStore(launcherBasePath(currentRoot), "win32");
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

  test("keeps explicit advanced overrides when they differ from installation-root defaults", async () => {
    const currentRoot = await createTempDir("override-workspace");
    const altWorkdir = await createTempDir("override-workdir");

    await createWorkspace(currentRoot);

    const store = new JsonLauncherSettingsStore(launcherBasePath(currentRoot), "win32");
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
  });

  test("stores worktree settings in the owner root data directory", async () => {
    const mainRoot = await createTempDir("shared-main-workspace");
    const worktreeRoot = path.join(mainRoot, ".worktrees", "web-cn-redesign");

    await createWorkspace(mainRoot);
    await createWorkspace(worktreeRoot);

    const store = new JsonLauncherSettingsStore(launcherBasePath(worktreeRoot), "win32");
    const loaded = await store.load();

    expect(loaded.installationRoot).toBe(worktreeRoot);
    expect(await fileExists(settingsPath(mainRoot))).toBe(true);
    expect(await fileExists(settingsPath(worktreeRoot))).toBe(false);
  });

  test("uses the saved installation root across worktree launches", async () => {
    const mainRoot = await createTempDir("pinned-main-workspace");
    const worktreeRoot = path.join(mainRoot, ".worktrees", "web-cn-redesign");

    await createWorkspace(mainRoot);
    await createWorkspace(worktreeRoot);

    const store = new JsonLauncherSettingsStore(launcherBasePath(worktreeRoot), "win32");
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
  });
});

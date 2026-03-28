import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, test } from "vitest";
import { JsonLauncherSettingsStore } from "@main/services/settings-store";

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

afterEach(async () => {
  await Promise.all(
    tempRoots.splice(0).map(async (target) => {
      await fs.rm(target, { recursive: true, force: true });
    }),
  );
});

describe("launcher settings store", () => {
  test("persists defaults only when the settings file is missing", async () => {
    const currentRoot = await createTempDir("default-workspace");
    const userDataPath = await createTempDir("default-userdata");

    await createWorkspace(currentRoot);

    const store = new JsonLauncherSettingsStore(userDataPath, path.join(currentRoot, "launcher"), "win32");
    const loaded = await store.load();

    expect(loaded.serverExecutablePath).toBe(path.join(currentRoot, "server", "raylea-server.exe"));
    expect(loaded.configPath).toBe(path.join(currentRoot, "config", "user.yaml"));
    expect(loaded.workdir).toBe(currentRoot);

    const saved = JSON.parse(await fs.readFile(path.join(userDataPath, "launcher.json"), "utf8")) as {
      serverExecutablePath: string;
      configPath: string;
      workdir: string;
    };
    expect(saved.serverExecutablePath).toBe(path.join(currentRoot, "server", "raylea-server.exe"));
    expect(saved.configPath).toBe(path.join(currentRoot, "config", "user.yaml"));
    expect(saved.workdir).toBe(currentRoot);
  });

  test("rebases stale workspace-relative paths onto the current workspace", async () => {
    const previousRoot = await createTempDir("old-workspace");
    const currentRoot = await createTempDir("current-workspace");
    const userDataPath = await createTempDir("userdata");

    await createWorkspace(currentRoot);
    await fs.mkdir(path.join(userDataPath), { recursive: true });
    await fs.writeFile(
      path.join(userDataPath, "launcher.json"),
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

    const store = new JsonLauncherSettingsStore(userDataPath, path.join(currentRoot, "launcher"), "win32");
    const loaded = await store.load();

    expect(loaded.serverExecutablePath).toBe(path.join(currentRoot, "server", "raylea-server.exe"));
    expect(loaded.configPath).toBe(path.join(currentRoot, "config", "user.yaml"));
    expect(loaded.workdir).toBe(currentRoot);
    expect(loaded.closeBehavior).toBe("hide_to_tray");

    const saved = JSON.parse(await fs.readFile(path.join(userDataPath, "launcher.json"), "utf8")) as {
      serverExecutablePath: string;
      configPath: string;
      workdir: string;
    };
    expect(saved.serverExecutablePath).toBe(path.join(currentRoot, "server", "raylea-server.exe"));
    expect(saved.configPath).toBe(path.join(currentRoot, "config", "user.yaml"));
    expect(saved.workdir).toBe(currentRoot);
  });

  test("accepts legacy PascalCase settings keys", async () => {
    const currentRoot = await createTempDir("legacy-workspace");
    const userDataPath = await createTempDir("legacy-userdata");

    await createWorkspace(currentRoot);
    await fs.writeFile(
      path.join(userDataPath, "launcher.json"),
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

    const store = new JsonLauncherSettingsStore(userDataPath, path.join(currentRoot, "launcher"), "win32");
    const loaded = await store.load();

    expect(loaded.serverExecutablePath).toBe(path.join(currentRoot, "server", "raylea-server.exe"));
    expect(loaded.configPath).toBe(path.join(currentRoot, "config", "user.yaml"));
    expect(loaded.workdir).toBe(currentRoot);
    expect(loaded.closeBehavior).toBe("hide_to_tray");
  });
});

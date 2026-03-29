import { EventEmitter } from "node:events";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, test, vi } from "vitest";
import {
  resolveConfigSchemaPath,
  ServerProcessController,
} from "@main/services/process-controller";
import type { LauncherSettings } from "@shared/launcher-models";

class FakeStream extends EventEmitter {
  setEncoding() {}
  pause() {}
  resume() {}
}

class FakeChildProcess extends EventEmitter {
  stdout = new FakeStream();
  stderr = new FakeStream();
  stdin = new FakeStream();
  stdio = [this.stdin, this.stdout, this.stderr] as const;
  pid = 4242;
  killed = false;
  exitCode: number | null = null;
  signalCode: NodeJS.Signals | null = null;
  spawnfile = "";
  spawnargs: string[] = [];

  kill = vi.fn((signal?: NodeJS.Signals | number) => {
    this.killed = true;
    if (typeof signal === "string") {
      this.signalCode = signal;
    }
    return true;
  });
}

const tempRoots: string[] = [];

async function createTempDir(label: string) {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), `raylea-${label}-`));
  tempRoots.push(tempRoot);
  return tempRoot;
}

function createSettings(root: string, workdir: string): LauncherSettings {
  return {
    serverExecutablePath: path.join(root, "server", "raylea-server.exe"),
    configPath: path.join(root, "config", "user.yaml"),
    workdir,
    closeBehavior: "ask_every_time",
  };
}

afterEach(async () => {
  await Promise.all(
    tempRoots.splice(0).map(async (target) => {
      await fs.rm(target, { recursive: true, force: true });
    }),
  );
});

describe("resolveConfigSchemaPath", () => {
  test("prefers the installation root over the runtime workdir", async () => {
    const installRoot = await createTempDir("install-root");
    const runtimeRoot = await createTempDir("runtime-root");

    await fs.mkdir(path.join(installRoot, "contracts"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "server"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "config"), { recursive: true });
    await fs.writeFile(path.join(installRoot, "contracts", "config.user.schema.json"), "{}", "utf8");
    await fs.writeFile(path.join(installRoot, "config", "user.yaml"), "server: {}\n", "utf8");

    const settings = createSettings(installRoot, runtimeRoot);

    await expect(resolveConfigSchemaPath(settings)).resolves.toBe(
      path.join(installRoot, "contracts", "config.user.schema.json"),
    );
  });
});

describe("ServerProcessController", () => {
  test("passes an explicit config schema path to the spawned server", async () => {
    const installRoot = await createTempDir("controller-start");
    const runtimeRoot = await createTempDir("controller-runtime");

    await fs.mkdir(path.join(installRoot, "contracts"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "server"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "config"), { recursive: true });
    await fs.writeFile(path.join(installRoot, "contracts", "config.user.schema.json"), "{}", "utf8");
    await fs.writeFile(path.join(installRoot, "config", "user.yaml"), "server: {}\n", "utf8");

    const child = new FakeChildProcess();
    const spawnProcess = vi.fn(() => {
      queueMicrotask(() => {
        child.emit("spawn");
      });
      return child as never;
    });

    const controller = new ServerProcessController({
      spawnProcess,
      fileSystem: {
        mkdir: vi.fn(async () => undefined),
        appendFile: vi.fn(async () => undefined),
      },
      terminateProcessId: vi.fn(async () => true),
    });

    await controller.start(createSettings(installRoot, runtimeRoot));

    expect(spawnProcess).toHaveBeenCalledWith(
      path.join(installRoot, "server", "raylea-server.exe"),
      [
        "-config",
        path.join(installRoot, "config", "user.yaml"),
        "-config-schema",
        path.join(installRoot, "contracts", "config.user.schema.json"),
      ],
      expect.objectContaining({
        cwd: runtimeRoot,
        windowsHide: true,
        stdio: "pipe",
      }),
    );
  });

  test("rejects spawn errors and records them for diagnostics", async () => {
    const installRoot = await createTempDir("controller-error");
    const runtimeRoot = await createTempDir("controller-error-runtime");

    await fs.mkdir(path.join(installRoot, "contracts"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "server"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "config"), { recursive: true });
    await fs.writeFile(path.join(installRoot, "contracts", "config.user.schema.json"), "{}", "utf8");
    await fs.writeFile(path.join(installRoot, "config", "user.yaml"), "server: {}\n", "utf8");

    const child = new FakeChildProcess();
    const spawnProcess = vi.fn(() => {
      queueMicrotask(() => {
        child.emit("error", new Error("spawn EACCES"));
      });
      return child as never;
    });

    const controller = new ServerProcessController({
      spawnProcess,
      fileSystem: {
        mkdir: vi.fn(async () => undefined),
        appendFile: vi.fn(async () => undefined),
      },
      terminateProcessId: vi.fn(async () => true),
    });

    await expect(controller.start(createSettings(installRoot, runtimeRoot))).rejects.toThrow("spawn EACCES");
    expect(controller.isRunning).toBe(false);
    expect(controller.getRecentStderr()).toContain("spawn EACCES");
  });

  test("treats a child with an exit code as no longer running", async () => {
    const installRoot = await createTempDir("controller-running");
    const runtimeRoot = await createTempDir("controller-running-runtime");

    await fs.mkdir(path.join(installRoot, "contracts"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "server"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "config"), { recursive: true });
    await fs.writeFile(path.join(installRoot, "contracts", "config.user.schema.json"), "{}", "utf8");
    await fs.writeFile(path.join(installRoot, "config", "user.yaml"), "server: {}\n", "utf8");

    const child = new FakeChildProcess();
    const spawnProcess = vi.fn(() => {
      queueMicrotask(() => {
        child.emit("spawn");
      });
      return child as never;
    });

    const controller = new ServerProcessController({
      spawnProcess,
      fileSystem: {
        mkdir: vi.fn(async () => undefined),
        appendFile: vi.fn(async () => undefined),
      },
      terminateProcessId: vi.fn(async () => true),
    });

    await controller.start(createSettings(installRoot, runtimeRoot));
    child.exitCode = 1;

    expect(controller.isRunning).toBe(false);
  });
});

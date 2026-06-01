import { EventEmitter } from "node:events";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, test, vi } from "vitest";
import { ServerProcessController } from "@main/services/process-controller";
import type { LauncherResolvedSettings } from "@shared/launcher-models";

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

function createSettings(root: string, workdir: string): LauncherResolvedSettings {
  return {
    installationRoot: root,
    serverExecutablePath: path.join(root, "server", "raylea-server.exe"),
    configPath: path.join(root, "config", "user.yaml"),
    workdir,
  };
}

function createFileSystemDouble() {
  return {
    mkdir: vi.fn(async () => undefined),
    appendFile: vi.fn(async () => undefined),
  };
}

function loggedPaths(fileSystem: ReturnType<typeof createFileSystemDouble>) {
  return fileSystem.appendFile.mock.calls.map(([target]) => target);
}

async function flushLogWrites() {
  await Promise.resolve();
  await Promise.resolve();
}

afterEach(async () => {
  await Promise.all(
    tempRoots.splice(0).map(async (target) => {
      await fs.rm(target, { recursive: true, force: true });
    }),
  );
});

describe("ServerProcessController", () => {
  test("spawns the server with the config path only", async () => {
    const installRoot = await createTempDir("controller-start");
    const runtimeRoot = await createTempDir("controller-runtime");

    await fs.mkdir(path.join(installRoot, "server"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "config"), { recursive: true });
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
      ["-config", path.join(installRoot, "config", "user.yaml")],
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

    await fs.mkdir(path.join(installRoot, "server"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "config"), { recursive: true });
    await fs.writeFile(path.join(installRoot, "config", "user.yaml"), "server: {}\n", "utf8");

    const child = new FakeChildProcess();
    const spawnProcess = vi.fn(() => {
      queueMicrotask(() => {
        child.emit("error", new Error("spawn EACCES"));
      });
      return child as never;
    });

    const fileSystem = createFileSystemDouble();
    const controller = new ServerProcessController({
      spawnProcess,
      fileSystem,
      terminateProcessId: vi.fn(async () => true),
    });

    await expect(controller.start(createSettings(installRoot, runtimeRoot))).rejects.toThrow("spawn EACCES");
    await flushLogWrites();
    expect(controller.isRunning).toBe(false);
    expect(controller.getRecentStderr()).toContain("spawn EACCES");
    expect(fileSystem.appendFile).toHaveBeenCalledWith(
      path.join(runtimeRoot, "logs", "launcher.log"),
      expect.stringContaining("spawn error: spawn EACCES"),
      "utf8",
    );
    expect(loggedPaths(fileSystem)).not.toContain(path.join(runtimeRoot, "logs", "server.log"));
  });

  test("treats a child with an exit code as no longer running", async () => {
    const installRoot = await createTempDir("controller-running");
    const runtimeRoot = await createTempDir("controller-running-runtime");

    await fs.mkdir(path.join(installRoot, "server"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "config"), { recursive: true });
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

  test("records non-zero exit codes for launcher diagnostics", async () => {
    const installRoot = await createTempDir("controller-exit-code");
    const runtimeRoot = await createTempDir("controller-exit-runtime");

    await fs.mkdir(path.join(installRoot, "server"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "config"), { recursive: true });
    await fs.writeFile(path.join(installRoot, "config", "user.yaml"), "server: {}\n", "utf8");

    const child = new FakeChildProcess();
    const spawnProcess = vi.fn(() => {
      queueMicrotask(() => {
        child.emit("spawn");
      });
      return child as never;
    });

    const fileSystem = createFileSystemDouble();
    const controller = new ServerProcessController({
      spawnProcess,
      fileSystem,
      terminateProcessId: vi.fn(async () => true),
    });

    await controller.start(createSettings(installRoot, runtimeRoot));
    child.exitCode = 23;
    child.emit("exit", 23, null);
    await flushLogWrites();

    expect(controller.getRecentStderr().join("\n")).toContain("23");
    expect(fileSystem.appendFile).toHaveBeenCalledWith(
      path.join(runtimeRoot, "logs", "launcher.log"),
      expect.stringContaining("退出码 23"),
      "utf8",
    );
    expect(loggedPaths(fileSystem)).not.toContain(path.join(runtimeRoot, "logs", "server.log"));
  });

  test("writes child stdout to server.log and keeps diagnostic extraction for launcher status", async () => {
    const installRoot = await createTempDir("controller-stdout");
    const runtimeRoot = await createTempDir("controller-stdout-runtime");

    await fs.mkdir(path.join(installRoot, "server"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "config"), { recursive: true });
    await fs.writeFile(path.join(installRoot, "config", "user.yaml"), "server: {}\n", "utf8");

    const child = new FakeChildProcess();
    const spawnProcess = vi.fn(() => {
      queueMicrotask(() => {
        child.emit("spawn");
      });
      return child as never;
    });

    const fileSystem = createFileSystemDouble();
    const controller = new ServerProcessController({
      spawnProcess,
      fileSystem,
      terminateProcessId: vi.fn(async () => true),
    });

    await controller.start(createSettings(installRoot, runtimeRoot));
    child.stdout.emit("data", "{\"level\":\"ERROR\",\"msg\":\"listen on 127.0.0.1:8080: bind: address already in use\"}\n");
    await flushLogWrites();

    expect(controller.getRecentStderr().join("\n")).toContain("listen on 127.0.0.1:8080");
    expect(fileSystem.appendFile).toHaveBeenCalledWith(
      path.join(runtimeRoot, "logs", "server.log"),
      expect.stringContaining("stdout: {\"level\":\"ERROR\""),
      "utf8",
    );
    expect(loggedPaths(fileSystem)).not.toContain(path.join(runtimeRoot, "logs", "launcher.log"));
  });

  test("writes child stderr to server.log and keeps stderr lines for launcher status", async () => {
    const installRoot = await createTempDir("controller-stderr");
    const runtimeRoot = await createTempDir("controller-stderr-runtime");

    await fs.mkdir(path.join(installRoot, "server"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "config"), { recursive: true });
    await fs.writeFile(path.join(installRoot, "config", "user.yaml"), "server: {}\n", "utf8");

    const child = new FakeChildProcess();
    const spawnProcess = vi.fn(() => {
      queueMicrotask(() => {
        child.emit("spawn");
      });
      return child as never;
    });

    const fileSystem = createFileSystemDouble();
    const controller = new ServerProcessController({
      spawnProcess,
      fileSystem,
      terminateProcessId: vi.fn(async () => true),
    });

    await controller.start(createSettings(installRoot, runtimeRoot));
    child.stderr.emit("data", "startup failed\n");
    await flushLogWrites();

    expect(controller.getRecentStderr()).toContain("startup failed");
    expect(fileSystem.appendFile).toHaveBeenCalledWith(
      path.join(runtimeRoot, "logs", "server.log"),
      expect.stringContaining("stderr: startup failed"),
      "utf8",
    );
    expect(loggedPaths(fileSystem)).not.toContain(path.join(runtimeRoot, "logs", "launcher.log"));
  });

  test("records force-kill fallback failures instead of swallowing them silently", async () => {
    const installRoot = await createTempDir("controller-force-kill");
    const runtimeRoot = await createTempDir("controller-force-kill-runtime");

    await fs.mkdir(path.join(installRoot, "server"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "config"), { recursive: true });
    await fs.writeFile(path.join(installRoot, "config", "user.yaml"), "server: {}\n", "utf8");

    const child = new FakeChildProcess();
    child.kill = vi.fn(() => {
      throw new Error("kill EPERM");
    });
    const spawnProcess = vi.fn(() => {
      queueMicrotask(() => {
        child.emit("spawn");
      });
      return child as never;
    });

    const fileSystem = createFileSystemDouble();
    const controller = new ServerProcessController({
      spawnProcess,
      fileSystem,
      terminateProcessId: vi.fn(async () => false),
    });

    await controller.start(createSettings(installRoot, runtimeRoot));
    await controller.forceKill();
    await flushLogWrites();

    expect(controller.getRecentStderr().join("\n")).toContain("kill EPERM");
    expect(fileSystem.appendFile).toHaveBeenCalledWith(
      path.join(runtimeRoot, "logs", "launcher.log"),
      expect.stringContaining("launcher: kill EPERM"),
      "utf8",
    );
    expect(loggedPaths(fileSystem)).not.toContain(path.join(runtimeRoot, "logs", "server.log"));
  });
});

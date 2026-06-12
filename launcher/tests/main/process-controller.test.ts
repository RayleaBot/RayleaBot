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
const fixedLogDate = new Date(2026, 5, 13, 12, 0, 0);
const fixedLogDay = "2026-06-13";

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

function datedLogPath(root: string, type: "server" | "launcher") {
  return path.join(root, "logs", type, `${fixedLogDay}.log`);
}

function legacyLogPath(root: string, type: "server" | "launcher") {
  return path.join(root, "logs", `${type}.log`);
}

async function flushLogWrites() {
  for (let index = 0; index < 6; index += 1) {
    await Promise.resolve();
  }
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
      now: () => fixedLogDate,
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
      now: () => fixedLogDate,
    });

    await expect(controller.start(createSettings(installRoot, runtimeRoot))).rejects.toThrow("spawn EACCES");
    await flushLogWrites();
    expect(controller.isRunning).toBe(false);
    expect(controller.getRecentStderr()).toContain("spawn EACCES");
    expect(fileSystem.mkdir).toHaveBeenCalledWith(path.join(runtimeRoot, "logs", "launcher"), { recursive: true });
    expect(fileSystem.appendFile).toHaveBeenCalledWith(
      datedLogPath(runtimeRoot, "launcher"),
      expect.stringContaining("spawn error: spawn EACCES"),
      "utf8",
    );
    expect(loggedPaths(fileSystem)).not.toContain(legacyLogPath(runtimeRoot, "server"));
    expect(loggedPaths(fileSystem)).not.toContain(legacyLogPath(runtimeRoot, "launcher"));
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
      now: () => fixedLogDate,
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
      now: () => fixedLogDate,
    });

    await controller.start(createSettings(installRoot, runtimeRoot));
    child.exitCode = 23;
    child.emit("exit", 23, null);
    await flushLogWrites();

    expect(controller.getRecentStderr().join("\n")).toContain("23");
    expect(fileSystem.mkdir).toHaveBeenCalledWith(path.join(runtimeRoot, "logs", "launcher"), { recursive: true });
    expect(fileSystem.appendFile).toHaveBeenCalledWith(
      datedLogPath(runtimeRoot, "launcher"),
      expect.stringContaining("退出码 23"),
      "utf8",
    );
    expect(loggedPaths(fileSystem)).not.toContain(legacyLogPath(runtimeRoot, "server"));
    expect(loggedPaths(fileSystem)).not.toContain(legacyLogPath(runtimeRoot, "launcher"));
  });

  test("writes child stdout to dated server log and keeps diagnostic extraction for launcher status", async () => {
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
      now: () => fixedLogDate,
    });

    await controller.start(createSettings(installRoot, runtimeRoot));
    child.stdout.emit("data", "{\"level\":\"ERROR\",\"msg\":\"listen on 127.0.0.1:8080: bind: address already in use\"}\n");
    await flushLogWrites();

    expect(controller.getRecentStderr().join("\n")).toContain("listen on 127.0.0.1:8080");
    expect(fileSystem.mkdir).toHaveBeenCalledWith(path.join(runtimeRoot, "logs", "server"), { recursive: true });
    expect(fileSystem.appendFile).toHaveBeenCalledWith(
      datedLogPath(runtimeRoot, "server"),
      expect.stringContaining("stdout: {\"level\":\"ERROR\""),
      "utf8",
    );
    expect(loggedPaths(fileSystem)).not.toContain(legacyLogPath(runtimeRoot, "server"));
    expect(loggedPaths(fileSystem)).not.toContain(legacyLogPath(runtimeRoot, "launcher"));
  });

  test("parses structured runtime preparation source probe from child stdout", async () => {
    const installRoot = await createTempDir("controller-runtime-progress");
    const runtimeRoot = await createTempDir("controller-runtime-progress-root");

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
      now: () => fixedLogDate,
    });

    await controller.start(createSettings(installRoot, runtimeRoot));
    child.stdout.emit("data", JSON.stringify({
      msg: "runtime_prepare_progress",
      ts: "2026-06-06T00:00:00Z",
      resource_kind: "chromium",
      label: "Chromium 浏览环境",
      resource_id: "chromium-windows-x64",
      version: "147.0.7727.24",
      source_label: "Chrome for Testing",
      source_url: "https://example.invalid/chrome.zip",
      archive_path: "C:\\RayleaBot\\cache\\downloads\\runtime\\chromium.zip",
      store_root: "C:\\RayleaBot\\.deps\\store\\chromium-windows-x64\\147.0.7727.24",
      stage: "probe",
      status: "running",
      progress: 0,
      summary: "正在测试 Chromium 浏览环境下载来源",
    }) + "\n");
    await flushLogWrites();

    const snapshot = controller.getRuntimePrepareSnapshot();
    expect(snapshot?.active).toBe(true);
    expect(snapshot?.summary).toBe("正在测试 Chromium 浏览环境下载来源");
    expect(snapshot?.resources[0]?.stage).toBe("probe");
    expect(snapshot?.resources[0]?.sourceUrl).toBe("https://example.invalid/chrome.zip");
    expect(snapshot?.resources[0]?.progress).toBe(0);
    expect(controller.getRecentStderr().join("\n")).not.toContain("runtime_prepare_progress");
    expect(fileSystem.appendFile).toHaveBeenCalledWith(
      datedLogPath(runtimeRoot, "server"),
      expect.stringContaining("runtime_prepare_progress"),
      "utf8",
    );
  });

  test("parses runtime preparation progress split across stdout chunks", async () => {
    const installRoot = await createTempDir("controller-runtime-progress-split");
    const runtimeRoot = await createTempDir("controller-runtime-progress-split-root");

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
      fileSystem: createFileSystemDouble(),
      terminateProcessId: vi.fn(async () => true),
      now: () => fixedLogDate,
    });

    await controller.start(createSettings(installRoot, runtimeRoot));
    const line = JSON.stringify({
      msg: "runtime_prepare_progress",
      resource_kind: "python-runtime",
      label: "Python 运行环境",
      stage: "extract",
      status: "running",
      progress: 51,
      summary: "正在解压 Python 运行环境",
    });
    child.stdout.emit("data", line.slice(0, 40));
    expect(controller.getRuntimePrepareSnapshot()).toBeNull();

    child.stdout.emit("data", `${line.slice(40)}\n`);

    const snapshot = controller.getRuntimePrepareSnapshot();
    expect(snapshot?.summary).toBe("正在解压 Python 运行环境");
    expect(snapshot?.resources[0]?.progress).toBe(51);
  });

  test("keeps ordinary runtime logs out of runtime preparation progress", async () => {
    const installRoot = await createTempDir("controller-runtime-ignore");
    const runtimeRoot = await createTempDir("controller-runtime-ignore-root");

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
      fileSystem: createFileSystemDouble(),
      terminateProcessId: vi.fn(async () => true),
      now: () => fixedLogDate,
    });

    await controller.start(createSettings(installRoot, runtimeRoot));
    child.stdout.emit("data", "{\"msg\":\"startup runtime prepare requested\",\"resource_kind\":\"chromium\"}\n");

    expect(controller.getRuntimePrepareSnapshot()).toBeNull();
  });

  test("writes child stderr to dated server log and keeps stderr lines for launcher status", async () => {
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
      now: () => fixedLogDate,
    });

    await controller.start(createSettings(installRoot, runtimeRoot));
    child.stderr.emit("data", "startup failed\n");
    await flushLogWrites();

    expect(controller.getRecentStderr()).toContain("startup failed");
    expect(fileSystem.mkdir).toHaveBeenCalledWith(path.join(runtimeRoot, "logs", "server"), { recursive: true });
    expect(fileSystem.appendFile).toHaveBeenCalledWith(
      datedLogPath(runtimeRoot, "server"),
      expect.stringContaining("stderr: startup failed"),
      "utf8",
    );
    expect(loggedPaths(fileSystem)).not.toContain(legacyLogPath(runtimeRoot, "server"));
    expect(loggedPaths(fileSystem)).not.toContain(legacyLogPath(runtimeRoot, "launcher"));
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
      now: () => fixedLogDate,
    });

    await controller.start(createSettings(installRoot, runtimeRoot));
    await controller.forceKill();
    await flushLogWrites();

    expect(controller.getRecentStderr().join("\n")).toContain("kill EPERM");
    expect(fileSystem.mkdir).toHaveBeenCalledWith(path.join(runtimeRoot, "logs", "launcher"), { recursive: true });
    expect(fileSystem.appendFile).toHaveBeenCalledWith(
      datedLogPath(runtimeRoot, "launcher"),
      expect.stringContaining("launcher: kill EPERM"),
      "utf8",
    );
    expect(loggedPaths(fileSystem)).not.toContain(legacyLogPath(runtimeRoot, "server"));
    expect(loggedPaths(fileSystem)).not.toContain(legacyLogPath(runtimeRoot, "launcher"));
  });
});

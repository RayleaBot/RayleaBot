import fs from "node:fs/promises";
import path from "node:path";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import type { LauncherResolvedSettings } from "../../shared/launcher-models";
import { terminateProcessId } from "./process-termination";

const MAX_STDERR_LINES = 40;

interface FileSystemLike {
  mkdir(path: string, options: { recursive: true }): Promise<string | undefined | void>;
  appendFile(path: string, data: string, encoding: BufferEncoding): Promise<void>;
}

interface ServerProcessControllerDependencies {
  spawnProcess?: typeof spawn;
  fileSystem?: FileSystemLike;
  terminateProcessId?: (pid: number) => Promise<boolean>;
}

export class ServerProcessController {
  private readonly spawnProcess: typeof spawn;
  private readonly fileSystem: FileSystemLike;
  private readonly terminateProcessId: (pid: number) => Promise<boolean>;
  private process: ChildProcessWithoutNullStreams | null = null;
  private stderrLines: string[] = [];
  private logWriteQueue = Promise.resolve();
  logDirectory = path.resolve(process.cwd(), "logs");

  constructor(dependencies: ServerProcessControllerDependencies = {}) {
    this.spawnProcess = dependencies.spawnProcess ?? spawn;
    this.fileSystem = dependencies.fileSystem ?? fs;
    this.terminateProcessId = dependencies.terminateProcessId ?? terminateProcessId;
  }

  get isRunning() {
    const child = this.process;
    return Boolean(child && child.exitCode === null && child.signalCode === null);
  }

  get processId() {
    return this.process?.pid ?? null;
  }

  getRecentStderr() {
    return [...this.stderrLines];
  }

  async start(settings: LauncherResolvedSettings) {
    if (this.isRunning) {
      return;
    }

    this.stderrLines = [];
    await this.fileSystem.mkdir(settings.workdir, { recursive: true });
    this.logDirectory = path.join(settings.workdir, "logs");
    await this.fileSystem.mkdir(this.logDirectory, { recursive: true });
    const launcherLogPath = this.getLauncherLogPath();
    const serverLogPath = this.getServerLogPath();
    const child = this.spawnProcess(settings.serverExecutablePath, ["-config", settings.configPath], {
      cwd: settings.workdir,
      windowsHide: true,
      stdio: "pipe",
    });
    this.process = child;

    child.stdout.on("data", (chunk) => {
      const text = String(chunk);
      this.queueLogWrite(serverLogPath, "stdout", text);
      this.recordStdoutDiagnostics(text);
    });

    child.stderr.on("data", (chunk) => {
      const text = String(chunk);
      this.queueLogWrite(serverLogPath, "stderr", text);
      this.recordStderr(text);
    });

    child.on("exit", (code, signal) => {
      if (this.process === child) {
        this.process = null;
      }

      if ((code ?? 0) === 0 && !signal) {
        return;
      }

      const detail =
        code !== null && code !== undefined
          ? `服务进程已退出，退出码 ${code}。`
          : `服务进程已退出，信号 ${signal ?? "unknown"}。`;
      this.recordLauncherDiagnostic(launcherLogPath, detail);
    });

    await new Promise<void>((resolve, reject) => {
      let settled = false;

      const finishResolve = () => {
        if (!settled) {
          settled = true;
          resolve();
        }
      };

      const finishReject = (error: Error) => {
        if (!settled) {
          settled = true;
          reject(error);
        }
      };

      child.once("spawn", () => {
        finishResolve();
      });

      child.once("error", (error) => {
        this.recordStderr(error.message);
        this.queueLogWrite(launcherLogPath, "launcher", `spawn error: ${error.message}\n`);
        if (this.process === child) {
          this.process = null;
        }
        finishReject(error instanceof Error ? error : new Error(String(error)));
      });

      child.once("exit", () => {
        if (this.process === child) {
          this.process = null;
        }
        finishResolve();
      });
    });
  }

  async forceKill() {
    const child = this.process;
    if (!child) {
      return;
    }

    if (!this.isRunning) {
      if (this.process === child) {
        this.process = null;
      }
      return;
    }

    const pid = child.pid;
    const terminated = pid === undefined ? false : await this.terminateProcessId(pid);
    if (!terminated && this.isRunning) {
      try {
        child.kill();
      } catch (error) {
        this.recordLauncherDiagnostic(
          this.getLauncherLogPath(),
          error instanceof Error ? error.message : String(error),
        );
      }
    }

    await this.waitForExit(child, 500);

    if (process.platform !== "win32" && this.isRunning) {
      try {
        child.kill("SIGKILL");
      } catch (error) {
        this.recordLauncherDiagnostic(
          this.getLauncherLogPath(),
          error instanceof Error ? error.message : String(error),
        );
      }
      await this.waitForExit(child, 500);
    }

    if (this.process === child && !this.isRunning) {
      this.process = null;
    }
  }

  private queueLogWrite(logPath: string, stream: "stdout" | "stderr" | "launcher", text: string) {
    const entry = `[${new Date().toISOString()}] ${stream}: ${text}`;
    this.logWriteQueue = this.logWriteQueue
      .catch(() => undefined)
      .then(() => this.fileSystem.appendFile(logPath, entry, "utf8"))
      .catch(() => undefined);
  }

  private recordStderr(text: string) {
    for (const line of text.split(/\r?\n/).filter(Boolean)) {
      this.stderrLines.push(line);
    }
    this.stderrLines = this.stderrLines.slice(-MAX_STDERR_LINES);
  }

  private recordStdoutDiagnostics(text: string) {
    for (const rawLine of text.split(/\r?\n/)) {
      const line = rawLine.trim();
      if (!line || !this.shouldCaptureStdoutDiagnostic(line)) {
        continue;
      }
      this.stderrLines.push(line);
    }
    this.stderrLines = this.stderrLines.slice(-MAX_STDERR_LINES);
  }

  private shouldCaptureStdoutDiagnostic(line: string) {
    return line.includes("\"level\":\"ERROR\"")
      || /\bpanic\b/i.test(line)
      || /\bfatal\b/i.test(line)
      || /\blisten on\b/i.test(line)
      || /\bbind:\b/i.test(line);
  }

  private recordLauncherDiagnostic(logPath: string, text: string) {
    this.recordStderr(text);
    this.queueLogWrite(logPath, "launcher", `${text}\n`);
  }

  private getLauncherLogPath() {
    return path.join(this.logDirectory, "launcher.log");
  }

  private getServerLogPath() {
    return path.join(this.logDirectory, "server.log");
  }

  private async waitForExit(child: ChildProcessWithoutNullStreams, timeoutMs: number) {
    if (child.exitCode !== null || child.signalCode !== null) {
      return;
    }

    await new Promise<void>((resolve) => {
      const onExit = () => {
        done();
      };

      const onError = () => {
        done();
      };

      const done = () => {
        clearTimeout(timer);
        child.off("exit", onExit);
        child.off("error", onError);
        resolve();
      };

      const timer = setTimeout(done, timeoutMs);

      child.once("exit", onExit);
      child.once("error", onError);
    });
  }
}

import fs from "node:fs/promises";
import path from "node:path";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import type {
  LauncherResolvedSettings,
  RuntimePrepareResourceProgress,
  RuntimePrepareSnapshot,
  RuntimePrepareStatus,
} from "../../shared/launcher-models";
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
  now?: () => Date;
}

type RuntimePrepareLogLine = {
  msg?: unknown;
  resource_kind?: unknown;
  label?: unknown;
  resource_id?: unknown;
  version?: unknown;
  source_label?: unknown;
  source_url?: unknown;
  archive_path?: unknown;
  store_root?: unknown;
  stage?: unknown;
  status?: unknown;
  progress?: unknown;
  downloaded_bytes?: unknown;
  total_bytes?: unknown;
  extracted_entries?: unknown;
  total_entries?: unknown;
  summary?: unknown;
  err?: unknown;
  ts?: unknown;
};

function stringValue(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function numberValue(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function normalizeProgress(value: unknown) {
  const progress = numberValue(value);
  if (progress === null) {
    return null;
  }
  if (progress < 0) {
    return 0;
  }
  if (progress > 100) {
    return 100;
  }
  return Math.round(progress);
}

function formatLocalLogDate(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function normalizeRuntimePrepareStatus(value: unknown): RuntimePrepareStatus {
  const status = stringValue(value);
  if (status === "running" || status === "succeeded" || status === "failed" || status === "pending") {
    return status;
  }
  return "running";
}

function parseRuntimePrepareProgressLine(line: string): RuntimePrepareResourceProgress | null {
  let parsed: RuntimePrepareLogLine;
  try {
    parsed = JSON.parse(line) as RuntimePrepareLogLine;
  } catch {
    return null;
  }
  if (parsed.msg !== "runtime_prepare_progress") {
    return null;
  }
  const kind = stringValue(parsed.resource_kind);
  if (!kind) {
    return null;
  }
  const label = stringValue(parsed.label) || kind;
  const status = normalizeRuntimePrepareStatus(parsed.status);
  return {
    kind,
    label,
    resourceId: stringValue(parsed.resource_id),
    version: stringValue(parsed.version),
    sourceLabel: stringValue(parsed.source_label),
    sourceUrl: stringValue(parsed.source_url),
    archivePath: stringValue(parsed.archive_path),
    storeRoot: stringValue(parsed.store_root),
    stage: stringValue(parsed.stage) || "inspect",
    status,
    progress: normalizeProgress(parsed.progress),
    downloadedBytes: numberValue(parsed.downloaded_bytes),
    totalBytes: numberValue(parsed.total_bytes),
    extractedEntries: numberValue(parsed.extracted_entries),
    totalEntries: numberValue(parsed.total_entries),
    summary: stringValue(parsed.summary) || `${label}${status === "failed" ? "准备失败" : "准备中"}`,
    error: stringValue(parsed.err),
    updatedAt: stringValue(parsed.ts) || new Date().toISOString(),
  };
}

export class ServerProcessController {
  private readonly spawnProcess: typeof spawn;
  private readonly fileSystem: FileSystemLike;
  private readonly terminateProcessId: (pid: number) => Promise<boolean>;
  private readonly now: () => Date;
  private process: ChildProcessWithoutNullStreams | null = null;
  private stderrLines: string[] = [];
  private runtimePrepareResources = new Map<string, RuntimePrepareResourceProgress>();
  private runtimePrepareCurrentKind = "";
  private runtimePrepareSummary = "";
  private runtimePrepareActive = false;
  private stdoutLineBuffer = "";
  private logWriteQueue = Promise.resolve();
  logDirectory = path.resolve(process.cwd(), "logs");

  constructor(dependencies: ServerProcessControllerDependencies = {}) {
    this.spawnProcess = dependencies.spawnProcess ?? spawn;
    this.fileSystem = dependencies.fileSystem ?? fs;
    this.terminateProcessId = dependencies.terminateProcessId ?? terminateProcessId;
    this.now = dependencies.now ?? (() => new Date());
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

  getRuntimePrepareSnapshot(): RuntimePrepareSnapshot | null {
    if (!this.runtimePrepareActive && this.runtimePrepareResources.size === 0) {
      return null;
    }
    return {
      active: this.runtimePrepareActive,
      currentKind: this.runtimePrepareCurrentKind,
      summary: this.runtimePrepareSummary,
      resources: [...this.runtimePrepareResources.values()],
    };
  }

  clearRuntimePrepareSnapshot() {
    this.runtimePrepareActive = false;
    this.runtimePrepareCurrentKind = "";
    this.runtimePrepareSummary = "";
    this.runtimePrepareResources.clear();
  }

  async start(settings: LauncherResolvedSettings) {
    if (this.isRunning) {
      return;
    }

    this.stderrLines = [];
    this.clearRuntimePrepareSnapshot();
    this.stdoutLineBuffer = "";
    await this.fileSystem.mkdir(settings.workdir, { recursive: true });
    this.logDirectory = path.join(settings.workdir, "logs");
    await this.fileSystem.mkdir(this.logDirectory, { recursive: true });
    const child = this.spawnProcess(settings.serverExecutablePath, ["-config", settings.configPath], {
      cwd: settings.workdir,
      windowsHide: true,
      stdio: "pipe",
    });
    this.process = child;

    child.stdout.on("data", (chunk) => {
      const text = String(chunk);
      this.queueLogWrite("server", "stdout", text);
      this.recordStdoutDiagnostics(text);
    });

    child.stderr.on("data", (chunk) => {
      const text = String(chunk);
      this.queueLogWrite("server", "stderr", text);
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
      this.recordLauncherDiagnostic(detail);
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
        this.queueLogWrite("launcher", "launcher", `spawn error: ${error.message}\n`);
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
          error instanceof Error ? error.message : String(error),
        );
      }
      await this.waitForExit(child, 500);
    }

    if (this.process === child && !this.isRunning) {
      this.process = null;
    }
  }

  private queueLogWrite(logType: "server" | "launcher", stream: "stdout" | "stderr" | "launcher", text: string) {
    const timestamp = this.now();
    const logPath = this.getLogPath(logType, timestamp);
    const entry = `[${timestamp.toISOString()}] ${stream}: ${text}`;
    this.logWriteQueue = this.logWriteQueue
      .catch(() => undefined)
      .then(async () => {
        await this.fileSystem.mkdir(path.dirname(logPath), { recursive: true });
        await this.fileSystem.appendFile(logPath, entry, "utf8");
      })
      .catch(() => undefined);
  }

  private recordStderr(text: string) {
    for (const line of text.split(/\r?\n/).filter(Boolean)) {
      this.stderrLines.push(line);
    }
    this.stderrLines = this.stderrLines.slice(-MAX_STDERR_LINES);
  }

  private recordStdoutDiagnostics(text: string) {
    const combined = this.stdoutLineBuffer + text;
    const lines = combined.split(/\r?\n/);
    this.stdoutLineBuffer = lines.pop() ?? "";
    if (this.stdoutLineBuffer.length > 1024 * 1024) {
      this.stdoutLineBuffer = "";
    }
    for (const rawLine of lines) {
      const line = rawLine.trim();
      if (!line) {
        continue;
      }
      this.recordRuntimePrepareProgress(line);
      if (!this.shouldCaptureStdoutDiagnostic(line)) {
        continue;
      }
      this.stderrLines.push(line);
    }
    this.stderrLines = this.stderrLines.slice(-MAX_STDERR_LINES);
  }

  private recordRuntimePrepareProgress(line: string) {
    const event = parseRuntimePrepareProgressLine(line);
    if (!event) {
      return;
    }
    this.runtimePrepareResources.set(event.kind, event);
    this.runtimePrepareCurrentKind = event.kind;
    this.runtimePrepareSummary = event.summary;
    this.runtimePrepareActive = event.status === "running"
      || [...this.runtimePrepareResources.values()].some((item) => item.status === "running");
  }

  private shouldCaptureStdoutDiagnostic(line: string) {
    return line.includes("\"level\":\"ERROR\"")
      || /\bpanic\b/i.test(line)
      || /\bfatal\b/i.test(line)
      || /\blisten on\b/i.test(line)
      || /\bbind:\b/i.test(line);
  }

  private recordLauncherDiagnostic(text: string) {
    this.recordStderr(text);
    this.queueLogWrite("launcher", "launcher", `${text}\n`);
  }

  private getLogPath(logType: "server" | "launcher", date: Date) {
    return path.join(this.logDirectory, logType, `${formatLocalLogDate(date)}.log`);
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

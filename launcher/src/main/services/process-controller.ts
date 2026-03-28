import fs from "node:fs/promises";
import path from "node:path";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import type { LauncherSettings } from "../../shared/launcher-models";

const MAX_STDERR_LINES = 40;

export class ServerProcessController {
  private process: ChildProcessWithoutNullStreams | null = null;
  private stderrLines: string[] = [];
  logDirectory = path.resolve(process.cwd(), "logs");

  get isRunning() {
    return Boolean(this.process && !this.process.killed);
  }

  get processId() {
    return this.process?.pid ?? null;
  }

  getRecentStderr() {
    return [...this.stderrLines];
  }

  async start(settings: LauncherSettings) {
    if (this.isRunning) {
      return;
    }

    await fs.mkdir(settings.workdir, { recursive: true });
    this.logDirectory = path.join(settings.workdir, "logs");
    await fs.mkdir(this.logDirectory, { recursive: true });
    const logPath = path.join(this.logDirectory, "launcher.log");

    this.process = spawn(settings.serverExecutablePath, ["-config", settings.configPath], {
      cwd: settings.workdir,
      windowsHide: true,
      stdio: "pipe",
    });

    this.process.stdout.on("data", async (chunk) => {
      await fs.appendFile(logPath, `[${new Date().toISOString()}] stdout: ${String(chunk)}`, "utf8");
    });

    this.process.stderr.on("data", async (chunk) => {
      const text = String(chunk);
      await fs.appendFile(logPath, `[${new Date().toISOString()}] stderr: ${text}`, "utf8");
      for (const line of text.split(/\r?\n/).filter(Boolean)) {
        this.stderrLines.push(line);
      }
      this.stderrLines = this.stderrLines.slice(-MAX_STDERR_LINES);
    });

    this.process.once("exit", () => {
      this.process = null;
    });
  }

  async forceKill() {
    if (!this.process) {
      return;
    }
    this.process.kill("SIGTERM");
    this.process = null;
  }
}

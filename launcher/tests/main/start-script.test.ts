import { execFile } from "node:child_process";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, test } from "vitest";

const execFileAsync = promisify(execFile);
const tempRoots: string[] = [];
const testDir = path.dirname(fileURLToPath(import.meta.url));
const repositoryRoot = path.resolve(testDir, "..", "..", "..");
const startScriptPath = path.join(repositoryRoot, "start.bat");
const windowsRoot = process.env.SystemRoot ?? process.env.WINDIR ?? "C:\\Windows";
const commandShell = process.env.ComSpec ?? path.join(windowsRoot, "System32", "cmd.exe");

async function createTempDir(label: string) {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), `raylea-start-${label}-`));
  tempRoots.push(tempRoot);
  return tempRoot;
}

async function writeNodeStub(binDir: string, logPath: string) {
  await fs.writeFile(
    path.join(binDir, "node.cmd"),
    [
      "@echo off",
      `>>"${logPath}" echo CWD=%CD%`,
      `>>"${logPath}" echo ARGS=%*`,
      `>>"${logPath}" echo PROFILE=%RAYLEA_START_PROFILE%`,
      "exit /b 0",
      "",
    ].join("\r\n"),
    "utf8",
  );
}

async function readLogLines(logPath: string) {
  const content = await fs.readFile(logPath, "utf8");
  return content
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
}

function startScriptTestEnv(binDir: string, extra: NodeJS.ProcessEnv = {}) {
  return {
    ComSpec: commandShell,
    PATH: binDir,
    SystemRoot: windowsRoot,
    TEMP: process.env.TEMP ?? os.tmpdir(),
    TMP: process.env.TMP ?? os.tmpdir(),
    WINDIR: windowsRoot,
    ...extra,
  };
}

afterEach(async () => {
  await Promise.all(
    tempRoots.splice(0).map(async (target) => {
      await fs.rm(target, { recursive: true, force: true });
    }),
  );
});

describe("start.bat", () => {
  test.runIf(process.platform === "win32")("delegates to the Node start orchestrator", async () => {
    const binDir = await createTempDir("bin");
    const logPath = path.join(await createTempDir("logs"), "commands.log");
    await writeNodeStub(binDir, logPath);

    await execFileAsync(commandShell, ["/d", "/c", startScriptPath, "--dry-run"], {
      cwd: repositoryRoot,
      env: startScriptTestEnv(binDir, {
        RAYLEA_START_SKIP_LAUNCH: "1",
      }),
      windowsHide: true,
      timeout: 15000,
    });

    expect(await readLogLines(logPath)).toEqual([
      `CWD=${repositoryRoot}`,
      "ARGS=scripts\\start-dev.mjs --dry-run",
      "PROFILE=",
    ]);
  }, 20000);

  test.runIf(process.platform === "win32")("keeps start profile available to the orchestrator", async () => {
    const binDir = await createTempDir("bin");
    const logPath = path.join(await createTempDir("logs"), "commands.log");
    await writeNodeStub(binDir, logPath);

    await execFileAsync(commandShell, ["/d", "/c", startScriptPath], {
      cwd: repositoryRoot,
      env: startScriptTestEnv(binDir, {
        RAYLEA_START_PROFILE: "build",
      }),
      windowsHide: true,
      timeout: 15000,
    });

    expect(await readLogLines(logPath)).toEqual([
      `CWD=${repositoryRoot}`,
      "ARGS=scripts\\start-dev.mjs",
      "PROFILE=build",
    ]);
  }, 20000);
});

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

async function createTempDir(label: string) {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), `raylea-start-${label}-`));
  tempRoots.push(tempRoot);
  return tempRoot;
}

async function writeCommandStub(binDir: string, commandName: string, logPath: string) {
  await fs.writeFile(
    path.join(binDir, `${commandName}.cmd`),
    [
      "@echo off",
      `>>"${logPath}" echo ${commandName} %*`,
      "exit /b 0",
      "",
    ].join("\r\n"),
    "utf8",
  );
}

afterEach(async () => {
  await Promise.all(
    tempRoots.splice(0).map(async (target) => {
      await fs.rm(target, { recursive: true, force: true });
    }),
  );
});

describe("start.bat", () => {
  test.runIf(process.platform === "win32")("builds web, server, and launcher before skipping launch", async () => {
    const binDir = await createTempDir("bin");
    const logPath = path.join(await createTempDir("logs"), "commands.log");

    await writeCommandStub(binDir, "pnpm", logPath);
    await writeCommandStub(binDir, "go", logPath);

    const result = await execFileAsync("cmd.exe", ["/d", "/c", startScriptPath], {
      cwd: repositoryRoot,
      env: {
        ...process.env,
        PATH: `${binDir};${process.env.PATH ?? ""}`,
        RAYLEA_START_SKIP_LAUNCH: "1",
      },
      windowsHide: true,
      timeout: 15000,
    });

    const logLines = (await fs.readFile(logPath, "utf8"))
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean);

    expect(result.stdout).toContain("[RayleaBot] Building web...");
    expect(result.stdout).toContain("[RayleaBot] Building server...");
    expect(logLines).toEqual([
      `pnpm --dir "${path.join(repositoryRoot, "web")}" install --frozen-lockfile`,
      `pnpm --dir "${path.join(repositoryRoot, "web")}" run build`,
      `go build -o "${path.join(repositoryRoot, "server", "raylea-server.exe")}" ./cmd/raylea-server`,
      `pnpm --dir "${path.join(repositoryRoot, "launcher")}" install --frozen-lockfile`,
      `pnpm --dir "${path.join(repositoryRoot, "launcher")}" run build:app`,
    ]);
  });
});

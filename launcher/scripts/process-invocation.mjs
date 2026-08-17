import fs from "node:fs";
import path from "node:path";
import process from "node:process";

const GO_EXECUTABLE_ENV = "RAYLEA_GO_EXECUTABLE";

export function createProcessInvocation(command, args, options = {}) {
  const platform = options.platform ?? process.platform;
  if (command === "go") {
    return {
      command: resolveGoExecutablePath({
        platform,
        env: options.env ?? process.env,
        fileExists: options.fileExists ?? fs.existsSync,
      }),
      args,
    };
  }
  return { command, args };
}

function resolveGoExecutablePath({ platform, env, fileExists }) {
  const pathApi = platform === "win32" ? path.win32 : path.posix;
  const configuredExecutable = stripQuotes(String(env[GO_EXECUTABLE_ENV] ?? "").trim());
  if (configuredExecutable) {
    if (!pathApi.isAbsolute(configuredExecutable) || !fileExists(configuredExecutable)) {
      throw new Error(`${GO_EXECUTABLE_ENV} does not point to an existing absolute executable: ${configuredExecutable}`);
    }
    return configuredExecutable;
  }

  const executableName = platform === "win32" ? "go.exe" : "go";
  const delimiter = platform === "win32" ? ";" : ":";
  const searchPath = String(env.PATH ?? env.Path ?? "");
  const candidates = searchPath
    .split(delimiter)
    .map((directory) => stripQuotes(directory.trim()))
    .filter(Boolean)
    .map((directory) => pathApi.join(directory, executableName));

  if (platform === "win32") {
    const programFiles = stripQuotes(String(env.ProgramFiles ?? env.PROGRAMFILES ?? "").trim());
    if (programFiles) {
      candidates.push(pathApi.join(programFiles, "Go", "bin", executableName));
    }
  }

  const executablePath = candidates.find((candidate) => fileExists(candidate));
  if (!executablePath) {
    throw new Error("Go executable was not found. Run python scripts/check-toolchain.py for installation guidance.");
  }
  return executablePath;
}

function stripQuotes(value) {
  return value.replace(/^"|"$/g, "");
}

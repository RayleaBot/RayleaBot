import { spawn } from "node:child_process";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import {
  createElectronBuilderInvocation,
  createWindowsEntryBuildInvocation,
  preparePackagedApp,
  stageWindowsLauncherRuntime,
} from "./build-package-support.mjs";

const root = path.resolve(import.meta.dirname, "..");
const expectedWindowsBundle = path.join(root, "dist", "package", "win-unpacked", "RayleaLauncher.exe");
const renameFailurePattern = /rename '.*electron\.exe' -> '.*RayleaLauncher\.exe'/i;

let combinedOutput = "";
const packagedAppDir = preparePackagedApp(root);
const builderInvocation = createElectronBuilderInvocation(root, process.env, packagedAppDir);

async function runInvocation(invocation) {
  return new Promise((resolve, reject) => {
    const child = spawn(invocation.command, invocation.args, invocation.options);
    child.once("error", reject);
    child.once("close", (code) => resolve(code ?? 1));
  });
}

let exitCode = 1;
try {
  exitCode = await new Promise((resolve, reject) => {
    const child = spawn(builderInvocation.command, builderInvocation.args, builderInvocation.options);

    child.stdout?.on("data", (chunk) => {
      const text = chunk.toString();
      combinedOutput += text;
      process.stdout.write(text);
    });

    child.stderr?.on("data", (chunk) => {
      const text = chunk.toString();
      combinedOutput += text;
      process.stderr.write(text);
    });

    child.once("error", reject);
    child.once("close", (code) => resolve(code ?? 1));
  });
} finally {
  builderInvocation.cleanup();
  await fs.rm(packagedAppDir, { force: true, recursive: true });
}

if (process.platform !== "win32") {
  process.exit(exitCode);
}

const bundleExists = await fs
  .access(expectedWindowsBundle)
  .then(() => true)
  .catch(() => false);
const knownRenameFalseNegative = exitCode !== 0 && bundleExists && renameFailurePattern.test(combinedOutput);
if (exitCode !== 0 && !knownRenameFalseNegative) {
  process.exit(exitCode);
}
if (knownRenameFalseNegative) {
  console.warn(
    "[launcher] electron-builder emitted the known Windows rename false negative after producing the unpacked bundle; treating the build as successful.",
  );
}

const entryBuildRoot = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-launcher-entry-"));
const entryExecutable = path.join(entryBuildRoot, "RayleaLauncher.exe");
try {
  const entryBuildExitCode = await runInvocation(
    createWindowsEntryBuildInvocation(root, entryExecutable, process.env),
  );
  if (entryBuildExitCode !== 0) {
    throw new Error(`building the Windows launcher entry failed with exit code ${entryBuildExitCode}`);
  }
  stageWindowsLauncherRuntime(path.dirname(expectedWindowsBundle), entryExecutable);
} finally {
  await fs.rm(entryBuildRoot, { force: true, recursive: true });
}

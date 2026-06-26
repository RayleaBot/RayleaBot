import { spawn } from "node:child_process";
import fs from "node:fs/promises";
import path from "node:path";
import { createElectronBuilderInvocation } from "./build-package-support.mjs";

const root = path.resolve(import.meta.dirname, "..");
const expectedWindowsBundle = path.join(root, "dist", "package", "win-unpacked", "RayleaLauncher.exe");
const renameFailurePattern = /rename '.*electron\.exe' -> '.*RayleaLauncher\.exe'/i;

let combinedOutput = "";
const builderInvocation = createElectronBuilderInvocation(root, process.env);

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
}

if (exitCode === 0) {
  process.exit(0);
}

if (process.platform !== "win32") {
  process.exit(exitCode);
}

const bundleExists = await fs
  .access(expectedWindowsBundle)
  .then(() => true)
  .catch(() => false);

if (!bundleExists || !renameFailurePattern.test(combinedOutput)) {
  process.exit(exitCode);
}

console.warn(
  "[launcher] electron-builder emitted the known Windows rename false negative after producing the unpacked bundle; treating the build as successful.",
);
process.exit(0);

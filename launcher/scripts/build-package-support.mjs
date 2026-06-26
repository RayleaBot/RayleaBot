import fs from "node:fs";
import os from "node:os";
import path from "node:path";

function createCorepackPnpmShim() {
  const shimDir = fs.mkdtempSync(path.join(os.tmpdir(), "rayleabot-pnpm-"));
  if (process.platform === "win32") {
    fs.writeFileSync(
      path.join(shimDir, "pnpm.cmd"),
      "@echo off\r\ncorepack pnpm %*\r\n",
      "utf8",
    );
    return shimDir;
  }

  const shimPath = path.join(shimDir, "pnpm");
  fs.writeFileSync(shimPath, "#!/usr/bin/env sh\nexec corepack pnpm \"$@\"\n", "utf8");
  fs.chmodSync(shimPath, 0o755);
  return shimDir;
}

export function createElectronBuilderInvocation(root, env = process.env) {
  const pnpmShimDir = createCorepackPnpmShim();
  const nextEnv = {
    ...env,
    PATH: [pnpmShimDir, env.PATH ?? ""].filter(Boolean).join(path.delimiter),
  };

  return {
    command: process.execPath,
    args: [
      "--disable-warning=DEP0190",
      path.join(root, "node_modules", "electron-builder", "cli.js"),
      "--dir",
    ],
    options: {
      cwd: root,
      env: nextEnv,
      shell: false,
      stdio: ["inherit", "pipe", "pipe"],
      windowsHide: true,
    },
    cleanup() {
      fs.rmSync(pnpmShimDir, { force: true, recursive: true });
    },
  };
}

import fs from "node:fs";
import os from "node:os";
import path from "node:path";

export const PACKAGED_RUNTIME_DEPENDENCIES = ["yaml"];
export const WINDOWS_LAUNCHER_RUNTIME_DIRECTORY = "launcher";

function shouldCopyRuntimeDependencyFile(dependency, dependencyRoot, candidate) {
  const relative = path.relative(dependencyRoot, candidate);
  if (relative.length === 0 || fs.statSync(candidate).isDirectory()) {
    return relative.length === 0 || relative.split(path.sep)[0] === "dist";
  }
  if (dependency === "yaml") {
    return (
      ["LICENSE", "package.json", "util.js"].includes(relative)
      || (relative.split(path.sep)[0] === "dist" && path.extname(relative) === ".js")
    );
  }
  return false;
}

export function createPackagedAppManifest(sourceManifest) {
  const dependencies = {};
  for (const name of PACKAGED_RUNTIME_DEPENDENCIES) {
    const version = sourceManifest.dependencies?.[name];
    if (typeof version !== "string" || version.length === 0) {
      throw new Error(`launcher runtime dependency is not declared: ${name}`);
    }
    dependencies[name] = version;
  }

  return {
    name: `${sourceManifest.name}-runtime`,
    version: sourceManifest.version,
    private: true,
    description: sourceManifest.description,
    author: sourceManifest.author,
    main: sourceManifest.main,
    dependencies,
  };
}

export function preparePackagedApp(root) {
  const sourceManifest = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8"));
  const appDir = fs.mkdtempSync(path.join(os.tmpdir(), "rayleabot-launcher-app-"));

  for (const directory of ["main", "preload", "renderer"]) {
    fs.cpSync(path.join(root, "dist", directory), path.join(appDir, "dist", directory), {
      recursive: true,
    });
  }

  const packagedManifest = createPackagedAppManifest(sourceManifest);
  fs.writeFileSync(path.join(appDir, "package.json"), `${JSON.stringify(packagedManifest, null, 2)}\n`, "utf8");

  for (const dependency of PACKAGED_RUNTIME_DEPENDENCIES) {
    const sourceDependency = path.join(root, "node_modules", dependency);
    const dependencyManifest = JSON.parse(
      fs.readFileSync(path.join(sourceDependency, "package.json"), "utf8"),
    );
    if (
      Object.keys(dependencyManifest.dependencies ?? {}).length > 0
      || Object.keys(dependencyManifest.optionalDependencies ?? {}).length > 0
    ) {
      throw new Error(`launcher runtime dependency must be self-contained: ${dependency}`);
    }
    fs.cpSync(
      sourceDependency,
      path.join(appDir, "node_modules", dependency),
      {
        dereference: true,
        filter: (candidate) => shouldCopyRuntimeDependencyFile(dependency, sourceDependency, candidate),
        recursive: true,
      },
    );
  }
  return appDir;
}

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

export function createElectronBuilderInvocation(root, env = process.env, appDir = null) {
  const pnpmShimDir = createCorepackPnpmShim();
  const electronDist = env.ELECTRON_OVERRIDE_DIST_PATH?.trim();
  const nextEnv = {
    ...env,
    PATH: [pnpmShimDir, env.PATH ?? ""].filter(Boolean).join(path.delimiter),
  };

  return {
    command: process.execPath,
    args: [
      "--disable-warning=DEP0190",
      path.join(root, "node_modules", "electron-builder", "cli.js"),
      ...(appDir == null ? [] : [`--config.directories.app=${appDir}`]),
      ...(electronDist ? [`--config.electronDist=${electronDist}`] : []),
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

export function createWindowsEntryBuildInvocation(root, outputPath, env = process.env) {
  return {
    command: "go",
    args: [
      "build",
      "-trimpath",
      "-ldflags=-H=windowsgui -s -w",
      "-o",
      outputPath,
      path.join(root, "native", "windows-entry", "main_windows.go"),
    ],
    options: {
      cwd: path.resolve(root, "..", "server"),
      env,
      shell: false,
      stdio: "inherit",
      windowsHide: true,
    },
  };
}

export function stageWindowsLauncherRuntime(bundleRoot, entryExecutable) {
  const electronExecutable = path.join(bundleRoot, "RayleaLauncher.exe");
  if (!fs.statSync(electronExecutable).isFile()) {
    throw new Error(`packaged Electron executable is missing: ${electronExecutable}`);
  }
  if (!fs.statSync(entryExecutable).isFile()) {
    throw new Error(`launcher entry executable is missing: ${entryExecutable}`);
  }

  const runtimeRoot = path.join(bundleRoot, WINDOWS_LAUNCHER_RUNTIME_DIRECTORY);
  if (fs.existsSync(runtimeRoot)) {
    throw new Error(`launcher runtime directory already exists: ${runtimeRoot}`);
  }
  fs.mkdirSync(runtimeRoot);
  for (const child of fs.readdirSync(bundleRoot)) {
    if (child === WINDOWS_LAUNCHER_RUNTIME_DIRECTORY) {
      continue;
    }
    fs.renameSync(path.join(bundleRoot, child), path.join(runtimeRoot, child));
  }
  fs.copyFileSync(entryExecutable, path.join(bundleRoot, "RayleaLauncher.exe"));
}

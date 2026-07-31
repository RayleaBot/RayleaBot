import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { describe, expect, test } from "vitest";
import {
  createElectronBuilderInvocation,
  createPackagedAppManifest,
  createWindowsEntryBuildInvocation,
  preparePackagedApp,
  stageWindowsLauncherRuntime,
} from "../../scripts/build-package-support.mjs";

describe("build-package support", () => {
  test("invokes electron-builder through node without shell-based argument concatenation", () => {
    const root = path.resolve(import.meta.dirname, "..", "..");
    const electronDist = path.join(root, "node_modules", "electron", "dist");
    const invocation = createElectronBuilderInvocation(root, {
      PATH: process.env.PATH ?? "",
      CUSTOM_TEST_ENV: "fixture",
      ELECTRON_OVERRIDE_DIST_PATH: electronDist,
    });

    expect(invocation.command).toBe(process.execPath);
    expect(invocation.args[0]).toBe("--disable-warning=DEP0190");
    expect(invocation.args.at(-1)).toBe("--dir");
    expect(invocation.args.some((item) => item.endsWith(path.join("electron-builder", "cli.js")))).toBe(true);
    expect(invocation.args).toContain(`--config.electronDist=${electronDist}`);
    expect(invocation.options.shell).not.toBe(true);
    expect(invocation.options.env.CUSTOM_TEST_ENV).toBe("fixture");
    expect(invocation.options.env.PATH.split(path.delimiter)[0]).toContain("rayleabot-pnpm-");
    expect(typeof invocation.cleanup).toBe("function");
    invocation.cleanup();
  });

  test("stages only dependencies required by the packaged main process", () => {
    const sourceManifest = {
      name: "@rayleabot/launcher",
      version: "0.1.0",
      private: true,
      description: "RayleaBot Electron desktop launcher",
      author: "RayleaBot",
      main: "dist/main/main/index.js",
      dependencies: {
        "@fluentui/react-components": "9.74.3",
        react: "18.3.1",
        yaml: "2.9.0",
      },
    };

    expect(createPackagedAppManifest(sourceManifest)).toEqual({
      name: "@rayleabot/launcher-runtime",
      version: "0.1.0",
      private: true,
      description: "RayleaBot Electron desktop launcher",
      author: "RayleaBot",
      main: "dist/main/main/index.js",
      dependencies: {
        yaml: "2.9.0",
      },
    });
  });

  test("prepares an isolated application directory for electron-builder", () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "rayleabot-package-app-"));
    try {
      fs.writeFileSync(
        path.join(root, "package.json"),
        JSON.stringify({
          name: "@rayleabot/launcher",
          version: "0.1.0",
          description: "RayleaBot Electron desktop launcher",
          author: "RayleaBot",
          main: "dist/main/main/index.js",
          dependencies: {
            react: "18.3.1",
            yaml: "2.9.0",
          },
        }),
      );
      for (const directory of ["main", "preload", "renderer"]) {
        const output = path.join(root, "dist", directory);
        fs.mkdirSync(output, { recursive: true });
        fs.writeFileSync(path.join(output, "index.js"), directory);
      }
      const yamlModule = path.join(root, "node_modules", "yaml");
      fs.mkdirSync(yamlModule, { recursive: true });
      fs.writeFileSync(
        path.join(yamlModule, "package.json"),
        '{"name":"yaml","version":"2.9.0","license":"ISC"}',
      );

      const appDir = preparePackagedApp(root);
      const manifest = JSON.parse(fs.readFileSync(path.join(appDir, "package.json"), "utf8"));

      expect(manifest.dependencies).toEqual({ yaml: "2.9.0" });
      expect(fs.existsSync(path.join(appDir, "dist", "main", "index.js"))).toBe(true);
      expect(fs.existsSync(path.join(appDir, "node_modules", "yaml", "package.json"))).toBe(true);
      expect(fs.existsSync(path.join(appDir, "node_modules", "react"))).toBe(false);
      fs.rmSync(appDir, { force: true, recursive: true });
    } finally {
      fs.rmSync(root, { force: true, recursive: true });
    }
  });

  test("builds the native Windows entry from the server Go module", () => {
    const root = path.resolve("C:\\", "RayleaBot", "launcher");
    const outputPath = path.join(root, "dist", "entry", "RayleaLauncher.exe");
    const invocation = createWindowsEntryBuildInvocation(root, outputPath, { PATH: "go-bin" });

    expect(invocation.command).toBe("go");
    expect(invocation.options.cwd).toBe(path.resolve(root, "..", "server"));
    expect(invocation.options.shell).toBe(false);
    expect(invocation.args).toContain(outputPath);
    expect(invocation.args.at(-1)).toBe(path.join(root, "native", "windows-entry", "main_windows.go"));
  });

  test("keeps only the native launcher entry at the Windows bundle root", () => {
    const temp = fs.mkdtempSync(path.join(os.tmpdir(), "raylea-launcher-layout-"));
    const bundleRoot = path.join(temp, "win-unpacked");
    const entryExecutable = path.join(temp, "RayleaLauncher.entry.exe");
    try {
      fs.mkdirSync(path.join(bundleRoot, "resources"), { recursive: true });
      fs.mkdirSync(path.join(bundleRoot, "locales"), { recursive: true });
      fs.writeFileSync(path.join(bundleRoot, "RayleaLauncher.exe"), "electron");
      fs.writeFileSync(path.join(bundleRoot, "resources", "app.asar"), "asar");
      fs.writeFileSync(path.join(bundleRoot, "locales", "zh-CN.pak"), "locale");
      fs.writeFileSync(path.join(bundleRoot, "libEGL.dll"), "dll");
      fs.writeFileSync(entryExecutable, "entry");

      stageWindowsLauncherRuntime(bundleRoot, entryExecutable);

      expect(fs.readFileSync(path.join(bundleRoot, "RayleaLauncher.exe"), "utf8")).toBe("entry");
      expect(fs.readFileSync(path.join(bundleRoot, "launcher", "RayleaLauncher.exe"), "utf8")).toBe("electron");
      expect(fs.existsSync(path.join(bundleRoot, "launcher", "resources", "app.asar"))).toBe(true);
      expect(fs.existsSync(path.join(bundleRoot, "launcher", "locales", "zh-CN.pak"))).toBe(true);
      expect(fs.existsSync(path.join(bundleRoot, "launcher", "libEGL.dll"))).toBe(true);
      expect(fs.existsSync(path.join(bundleRoot, "resources"))).toBe(false);
      expect(fs.existsSync(path.join(bundleRoot, "locales"))).toBe(false);
      expect(fs.existsSync(path.join(bundleRoot, "libEGL.dll"))).toBe(false);
    } finally {
      fs.rmSync(temp, { force: true, recursive: true });
    }
  });
});

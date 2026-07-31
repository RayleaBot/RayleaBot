import path from "node:path";
import { describe, expect, test } from "vitest";
import { resolveLauncherAssetPaths, resolveLauncherBasePath } from "@main/services/app-paths";

describe("launcher app paths", () => {
  test("resolves preload and renderer assets from the application root", () => {
    const appPath = path.win32.join("C:\\", "RayleaBot", "launcher");

    expect(resolveLauncherAssetPaths(appPath)).toEqual({
      preloadPath: path.win32.join(appPath, "dist", "preload", "index.js"),
      rendererPath: path.win32.join(appPath, "dist", "renderer", "index.html"),
    });
  });

  test("uses the executable directory as the packaged installation root", () => {
    const appPath = path.win32.join("C:\\", "RayleaBot", "launcher");
    const executablePath = path.win32.join("C:\\", "Program Files", "RayleaLauncher", "RayleaLauncher.exe");

    expect(
      resolveLauncherBasePath({
        appPath,
        executablePath,
        isPackaged: true,
      }),
    ).toBe(path.win32.join("C:\\", "Program Files", "RayleaLauncher"));

    expect(
      resolveLauncherBasePath({
        appPath,
        executablePath,
        isPackaged: false,
      }),
    ).toBe(path.win32.join("C:\\", "RayleaBot", "launcher"));
  });

  test("uses the parent of the packaged Windows Electron runtime as the installation root", () => {
    const installRoot = path.win32.join("C:\\", "RayleaBot");
    const executablePath = path.win32.join(installRoot, "launcher", "RayleaLauncher.exe");

    expect(
      resolveLauncherBasePath({
        appPath: path.win32.join(executablePath, "resources", "app.asar"),
        executablePath,
        isPackaged: true,
      }),
    ).toBe(installRoot);
  });
});

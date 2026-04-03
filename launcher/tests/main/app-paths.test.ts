import path from "node:path";
import { describe, expect, test } from "vitest";
import { resolveLauncherAssetPaths, resolveLauncherBasePath } from "@main/services/app-paths";

describe("launcher app paths", () => {
  test("resolves preload and renderer assets from the application root", () => {
    const appPath = path.join("C:\\", "RayleaBot", "launcher");

    expect(resolveLauncherAssetPaths(appPath)).toEqual({
      preloadPath: path.join(appPath, "dist", "preload", "preload", "index.js"),
      rendererPath: path.join(appPath, "dist", "renderer", "index.html"),
    });
  });

  test("uses the executable directory as the packaged installation root", () => {
    expect(
      resolveLauncherBasePath({
        appPath: path.join("C:\\", "RayleaBot", "launcher"),
        executablePath: path.join("C:\\", "Program Files", "RayleaLauncher", "RayleaLauncher.exe"),
        isPackaged: true,
      }),
    ).toBe(path.join("C:\\", "Program Files", "RayleaLauncher"));

    expect(
      resolveLauncherBasePath({
        appPath: path.join("C:\\", "RayleaBot", "launcher"),
        executablePath: path.join("C:\\", "Program Files", "RayleaLauncher", "RayleaLauncher.exe"),
        isPackaged: false,
      }),
    ).toBe(path.join("C:\\", "RayleaBot", "launcher"));
  });
});

import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { describe, expect, test } from "vitest";
import { inspectEnvironmentFromNode, inspectLauncherEnvironment } from "@main/services/environment";
import type { LauncherResolvedSettings } from "@shared/launcher-models";

describe("inspectLauncherEnvironment", () => {
  test("reports bootstrap_available when user config is missing but default template exists", async () => {
    const inspection = await inspectLauncherEnvironment({
      serverExecutableExists: true,
      userConfigExists: false,
      defaultConfigExists: true,
      workdirWritable: true,
    });

    expect(inspection.hasBlockingIssues).toBe(false);
    expect(inspection.canBootstrapUserConfig).toBe(true);
    expect(inspection.checks.some((item) => item.code === "config.bootstrap_available")).toBe(true);
    expect(inspection.advisoryChecks).toEqual([]);
    expect(inspection.preflightChecks).toEqual(inspection.checks);
    expect(inspection.checks.every((item) => item.scope === "preflight")).toBe(true);
  });

  test("reports blocking preflight issues for missing local launch prerequisites", async () => {
    const inspection = await inspectLauncherEnvironment({
      installationRootExists: false,
      launcherSettingsResolved: false,
      serverExecutableExists: false,
      userConfigExists: true,
      defaultConfigExists: true,
      workdirWritable: false,
    });

    expect(inspection.hasBlockingIssues).toBe(true);
    expect(inspection.canBootstrapUserConfig).toBe(false);
    expect(inspection.checks.some((item) => item.code === "launcher.installation_root_missing")).toBe(true);
    expect(inspection.checks.some((item) => item.code === "launcher.settings_invalid")).toBe(true);
    expect(inspection.checks.some((item) => item.code === "server.executable_missing")).toBe(true);
    expect(inspection.checks.some((item) => item.code === "workdir.unwritable")).toBe(true);
  });

  test("reports config.missing when neither user nor default config is available", async () => {
    const inspection = await inspectLauncherEnvironment({
      serverExecutableExists: true,
      userConfigExists: false,
      defaultConfigExists: false,
      workdirWritable: true,
    });

    expect(inspection.hasBlockingIssues).toBe(true);
    expect(inspection.canBootstrapUserConfig).toBe(false);
    expect(inspection.checks.some((item) => item.code === "config.missing")).toBe(true);
  });

  test("inspects local preflight inputs and managed dependencies from the installation root", async () => {
    const installRoot = await fs.mkdtemp(path.join(os.tmpdir(), "rayleabot-install-"));
    const workdir = await fs.mkdtemp(path.join(os.tmpdir(), "rayleabot-workdir-"));
    const configDir = path.join(installRoot, "config");
    const serverExecutablePath = path.join(installRoot, process.platform === "win32" ? "raylea-server.exe" : "raylea-server");
    await fs.mkdir(configDir, { recursive: true });
    await fs.writeFile(serverExecutablePath, "", "utf8");
    await fs.writeFile(path.join(configDir, "default.yaml"), "server:\n  host: 127.0.0.1\n  port: 8080\n", "utf8");
    const manifest = buildDepsManifest(currentManifestPlatform());
    await fs.mkdir(path.join(installRoot, ".deps"), { recursive: true });
    await fs.writeFile(path.join(installRoot, ".deps", "manifest.json"), manifest, "utf8");
    await fs.mkdir(path.join(installRoot, "cache", "downloads", "runtime"), { recursive: true });
    await fs.writeFile(path.join(installRoot, "cache", "downloads", "runtime", "chromium-test-147.0.7727.24.zip"), "", "utf8");
    await fs.writeFile(
      path.join(installRoot, "cache", "downloads", "runtime", "python-test-3.12.13.tar.gz"),
      "",
      "utf8",
    );
    await fs.mkdir(path.join(installRoot, ".deps", "store", "python-test", "3.12.13", "python"), { recursive: true });
    await fs.writeFile(path.join(installRoot, ".deps", "store", "python-test", "3.12.13", "python", "python.exe"), "", "utf8");

    const settings: LauncherResolvedSettings = {
      installationRoot: installRoot,
      serverExecutablePath,
      configPath: path.join(configDir, "user.yaml"),
      workdir,
    };

    try {
      const inspection = await inspectEnvironmentFromNode(settings);

      expect(inspection.hasBlockingIssues).toBe(false);
      expect(inspection.canBootstrapUserConfig).toBe(true);
      expect(inspection.checks.some((item) => item.code === "launcher.installation_root")).toBe(true);
      expect(inspection.checks.some((item) => item.code === "server.executable")).toBe(true);
      expect(inspection.checks.some((item) => item.code === "config.bootstrap_available")).toBe(true);
      expect(inspection.checks.some((item) => item.code === "workdir.ready")).toBe(true);
      expect(inspection.checks.some((item) => item.code === "deps.manifest")).toBe(true);
      expect(inspection.checks.some((item) => item.code === "chromium.not_ready")).toBe(true);
      expect(inspection.checks.some((item) => item.code === "python.ready")).toBe(true);
      expect(inspection.checks.some((item) => item.code === "nodejs.not_ready")).toBe(true);
      expect(inspection.checks.find((item) => item.code === "chromium.not_ready")?.summary).toBe("已下载，未解压。");
      expect(inspection.checks.find((item) => item.code === "chromium.not_ready")?.detail).toContain("下载位置：");
      expect(inspection.checks.find((item) => item.code === "chromium.not_ready")?.detail).toContain("解压位置：");
      expect(inspection.checks.every((item) => item.scope === "preflight")).toBe(true);
    } finally {
      await fs.rm(installRoot, { recursive: true, force: true });
      await fs.rm(workdir, { recursive: true, force: true });
    }
  });

  test("reports only local preflight errors when the executable or workdir is invalid", async () => {
    const installRoot = await fs.mkdtemp(path.join(os.tmpdir(), "rayleabot-preflight-"));
    const configDir = path.join(installRoot, "config");
    const workdirBlockerPath = path.join(installRoot, "workdir-blocker");
    await fs.mkdir(configDir, { recursive: true });
    await fs.writeFile(path.join(configDir, "user.yaml"), "server:\n  host: 127.0.0.1\n  port: 8080\n", "utf8");
    await fs.writeFile(path.join(configDir, "default.yaml"), "server:\n  host: 127.0.0.1\n  port: 8080\n", "utf8");
    await fs.writeFile(workdirBlockerPath, "not-a-directory", "utf8");

    const settings: LauncherResolvedSettings = {
      installationRoot: installRoot,
      serverExecutablePath: path.join(installRoot, process.platform === "win32" ? "raylea-server.exe" : "raylea-server"),
      configPath: path.join(configDir, "user.yaml"),
      workdir: workdirBlockerPath,
    };

    try {
      const inspection = await inspectEnvironmentFromNode(settings);

      expect(inspection.hasBlockingIssues).toBe(true);
      expect(inspection.checks.some((item) => item.code === "server.executable_missing")).toBe(true);
      expect(inspection.checks.some((item) => item.code === "workdir.unwritable")).toBe(true);
      expect(inspection.checks.some((item) => item.code === "deps.manifest_missing")).toBe(true);
      expect(inspection.advisoryChecks).toEqual([]);
    } finally {
      await fs.rm(installRoot, { recursive: true, force: true });
    }
  });

  test("reports interrupted extraction when only a hidden runtime temp root exists", async () => {
    const installRoot = await fs.mkdtemp(path.join(os.tmpdir(), "rayleabot-deps-temp-"));
    const manifest = buildDepsManifest(currentManifestPlatform());
    await fs.mkdir(path.join(installRoot, ".deps"), { recursive: true });
    await fs.writeFile(path.join(installRoot, ".deps", "manifest.json"), manifest, "utf8");
    await fs.mkdir(path.join(installRoot, "cache", "downloads", "runtime"), { recursive: true });
    await fs.writeFile(path.join(installRoot, "cache", "downloads", "runtime", "chromium-test-147.0.7727.24.zip"), "", "utf8");
    const tempRoot = path.join(
      installRoot,
      ".deps",
      "store",
      "chromium-test",
      ".chromium-test-147.0.7727.24-interrupted",
    );
    await fs.mkdir(path.join(tempRoot, "chrome-win64"), { recursive: true });
    await fs.writeFile(path.join(tempRoot, "chrome-win64", "chrome.dll"), "", "utf8");

    const settings: LauncherResolvedSettings = {
      installationRoot: installRoot,
      serverExecutablePath: path.join(installRoot, process.platform === "win32" ? "raylea-server.exe" : "raylea-server"),
      configPath: path.join(installRoot, "config", "user.yaml"),
      workdir: await fs.mkdtemp(path.join(os.tmpdir(), "rayleabot-workdir-")),
    };

    try {
      const inspection = await inspectEnvironmentFromNode(settings);
      const chromiumCheck = inspection.checks.find((item) => item.code === "chromium.extract_incomplete");

      expect(chromiumCheck?.summary).toBe("上次解压未完成。");
      expect(chromiumCheck?.detail).toContain("下载位置：");
      expect(chromiumCheck?.detail).toContain("解压位置：");
      expect(chromiumCheck?.detail).toContain("临时目录：");
      expect(chromiumCheck?.detail).toContain(tempRoot);
      expect(inspection.checks.some((item) => item.code === "chromium.entrypoint_missing")).toBe(false);
    } finally {
      await fs.rm(installRoot, { recursive: true, force: true });
      await fs.rm(settings.workdir, { recursive: true, force: true });
    }
  });

  test("keeps runtime warning summaries separate from their titles", async () => {
    const inspection = await inspectLauncherEnvironment({
      serverExecutableExists: true,
      userConfigExists: true,
      defaultConfigExists: true,
      workdirWritable: true,
      depsManifestExists: true,
      depsManifestText: buildDepsManifest("windows-x64"),
      platform: "windows-x64",
      runtimeResourceStates: {
        "python-runtime": {
          archivePath: "C:\\RayleaBot\\cache\\downloads\\runtime\\python-test-3.12.13.tar.gz",
          archiveExists: true,
          storeRoot: "C:\\RayleaBot\\.deps\\store\\python-test\\3.12.13",
          storeRootExists: true,
          tempRootPaths: [],
          preparedStorePresent: false,
          missingEntrypoints: ["python"],
          primaryEntrypoint: "",
        },
      },
    });

    const pythonCheck = inspection.checks.find((item) => item.code === "python.entrypoint_missing");
    expect(pythonCheck?.title).toBe("Python 依赖");
    expect(pythonCheck?.summary).toBe("已解压，但入口文件缺失。");
    expect(pythonCheck?.summary).not.toContain("Python 依赖");
  });
});

function currentManifestPlatform() {
  const platform = process.platform === "win32" ? "windows" : process.platform === "darwin" ? "macos" : process.platform;
  const arch = process.arch === "x64" ? "x64" : process.arch;
  return `${platform}-${arch}`;
}

function buildDepsManifest(platform: string) {
  return `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "chromium-test",
      "kind": "chromium",
      "version": "147.0.7727.24",
      "platform": "${platform}",
      "sources": [{ "url": "https://example.invalid/chromium.zip", "kind": "upstream" }],
      "sha256": "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
      "archive_format": "zip",
      "entrypoints": { "browser": ["chrome-win64/chrome.exe"] }
    },
    {
      "id": "python-test",
      "kind": "python-runtime",
      "version": "3.12.13",
      "platform": "${platform}",
      "sources": [{ "url": "https://example.invalid/python.tar.gz", "kind": "upstream" }],
      "sha256": "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
      "archive_format": "tar.gz",
      "entrypoints": {
        "python": ["python/python.exe"]
      }
    },
    {
      "id": "node-test",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "${platform}",
      "sources": [{ "url": "https://example.invalid/node.zip", "kind": "upstream" }],
      "sha256": "313fa40c0d7b18575821de8cb17483031fe07d95de5994f6f435f3b345f85c66",
      "archive_format": "zip",
      "entrypoints": {
        "node": ["node/node.exe"],
        "npm": ["node/npm.cmd"]
      }
    }
  ]
}`;
}

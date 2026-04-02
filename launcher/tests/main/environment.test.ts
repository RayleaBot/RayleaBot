import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { describe, expect, test } from "vitest";
import {
  detectWindowsLongPathsStatus,
  inspectLauncherEnvironment,
  inspectEnvironmentFromNode,
} from "@main/services/environment";
import type { LauncherResolvedSettings } from "@shared/launcher-models";

describe("inspectLauncherEnvironment", () => {
  test("reports bootstrap_available when user config is missing but default template exists", async () => {
    const inspection = await inspectLauncherEnvironment({
      serverExecutableExists: true,
      userConfigExists: false,
      defaultConfigExists: true,
      workdirWritable: true,
      depsManifestExists: true,
      depsManifestText: JSON.stringify({
        manifest_version: 2,
        resources: [
          {
            platform: "windows-x64",
            kind: "chromium",
            source: "https://example.invalid/chromium.zip",
            sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
            archive_format: "zip",
            entrypoints: { browser: ["chrome-win64/chrome.exe"] },
          },
          {
            platform: "windows-x64",
            kind: "nodejs-runtime",
            source: "https://example.invalid/node.zip",
            sha256: "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
            archive_format: "zip",
            entrypoints: { node: ["node/node.exe"], npm: ["node/npm.cmd"] },
          },
          {
            platform: "windows-x64",
            kind: "python-runtime",
            source: "https://example.invalid/python.tar.gz",
            sha256: "10b9fd9ba9441f246f2cb279c2c6e6b2f98e60ef7960c313fd2bbc7f0c1e6f5e",
            archive_format: "tar.gz",
            entrypoints: { python: ["python/install/python.exe"], pip: ["python/install/Scripts/pip.exe"] },
          },
        ],
      }),
      templatesExist: true,
      templatesHaveFiles: true,
      platform: "windows-x64",
      longPaths: "enabled",
    });

    expect(inspection.hasBlockingIssues).toBe(false);
    expect(inspection.canBootstrapUserConfig).toBe(true);
    expect(inspection.checks.some((item) => item.code === "config.bootstrap_available")).toBe(true);
  });

  test("fails current platform when deps manifest misses matching resources", async () => {
    const inspection = await inspectLauncherEnvironment({
      serverExecutableExists: true,
      userConfigExists: true,
      defaultConfigExists: true,
      workdirWritable: true,
      depsManifestExists: true,
      depsManifestText: JSON.stringify({
        manifest_version: 2,
        resources: [{ platform: "windows-x64", kind: "chromium", archive_format: "zip", source: "https://example.invalid/chromium.zip", sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e", entrypoints: { browser: ["chrome-win64/chrome.exe"] } }],
      }),
      templatesExist: true,
      templatesHaveFiles: true,
      platform: "macos-arm64",
      longPaths: "unsupported",
    });

    expect(inspection.checks.some((item) => item.code === "deps.manifest_platform_missing")).toBe(true);
  });

  test("flags missing chromium resource when current platform only has other runtimes", async () => {
    const inspection = await inspectLauncherEnvironment({
      serverExecutableExists: true,
      userConfigExists: true,
      defaultConfigExists: true,
      workdirWritable: true,
      depsManifestExists: true,
      depsManifestText: JSON.stringify({
        manifest_version: 2,
        resources: [
          {
            id: "python-windows-x64",
            platform: "windows-x64",
            kind: "python-runtime",
            version: "3.12.13",
            source: "https://example.invalid/python.zip",
            sha256: "deadbeef",
            archive_format: "zip",
            entrypoints: { python: ["python/install/python.exe"], pip: ["python/install/Scripts/pip.exe"] },
          },
        ],
      }),
      templatesExist: true,
      templatesHaveFiles: true,
      platform: "windows-x64",
      longPaths: "enabled",
    });

    expect(inspection.checks.some((item) => item.code === "deps.chromium_missing")).toBe(true);
  });

  test("flags incomplete Python and Node runtime metadata from the deps manifest", async () => {
    const inspection = await inspectLauncherEnvironment({
      serverExecutableExists: true,
      userConfigExists: true,
      defaultConfigExists: true,
      workdirWritable: true,
      depsManifestExists: true,
      depsManifestText: JSON.stringify({
        manifest_version: 2,
        resources: [
          {
            id: "chromium-windows-x64",
            platform: "windows-x64",
            kind: "chromium",
            version: "147.0.7727.24",
            source: "https://storage.googleapis.com/chrome-for-testing-public/147.0.7727.24/win64/chrome-win64.zip",
            sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
            archive_format: "zip",
            entrypoints: { browser: ["chrome-win64/chrome.exe"] },
          },
          {
            id: "python-windows-x64",
            platform: "windows-x64",
            kind: "python-runtime",
            version: "3.12.13",
            source: "TODO(v0.1-phase0)",
            sha256: "TODO(v0.1-phase0)",
            archive_format: "tar.gz",
            entrypoints: { python: ["python/install/python.exe"], pip: ["python/install/Scripts/pip.exe"] },
          },
          {
            id: "nodejs-windows-x64",
            platform: "windows-x64",
            kind: "nodejs-runtime",
            version: "24.14.0",
            source: "https://nodejs.org/download/release/v24.14.0/node-v24.14.0-win-x64.zip",
            sha256: "deadbeef",
            archive_format: "zip",
            entrypoints: { node: ["node-v24.14.0-win-x64/node.exe"], npm: ["node-v24.14.0-win-x64/npm.cmd"] },
          },
        ],
      }),
      templatesExist: true,
      templatesHaveFiles: true,
      platform: "windows-x64",
      longPaths: "enabled",
    });

    expect(inspection.checks.some((item) => item.code === "deps.python_runtime_metadata_incomplete")).toBe(true);
    expect(inspection.checks.some((item) => item.code === "deps.nodejs_runtime_metadata_incomplete")).toBe(true);
  });

  test("flags missing template directory for render resources", async () => {
    const inspection = await inspectLauncherEnvironment({
      serverExecutableExists: true,
      userConfigExists: true,
      defaultConfigExists: true,
      workdirWritable: true,
      depsManifestExists: true,
      depsManifestText: JSON.stringify({
        manifest_version: 2,
        resources: [
          {
            id: "chromium-windows-x64",
            platform: "windows-x64",
            kind: "chromium",
            version: "147.0.7727.24",
            source: "https://storage.googleapis.com/chrome-for-testing-public/147.0.7727.24/win64/chrome-win64.zip",
            sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
            archive_format: "zip",
            entrypoints: { browser: ["chrome-win64/chrome.exe"] },
          },
        ],
      }),
      templatesExist: false,
      templatesHaveFiles: false,
      platform: "windows-x64",
      longPaths: "enabled",
    });

    expect(inspection.checks.some((item) => item.code === "render.templates_missing")).toBe(true);
  });

  test("detects enabled Windows long path support from registry output", async () => {
    const status = await detectWindowsLongPathsStatus(async () => ({
      stdout: [
        "HKEY_LOCAL_MACHINE\\SYSTEM\\CurrentControlSet\\Control\\FileSystem",
        "    LongPathsEnabled    REG_DWORD    0x1",
      ].join("\r\n"),
      stderr: "",
    }));

    expect(status).toBe("enabled");
  });

  test("detects disabled Windows long path support from registry output", async () => {
    const status = await detectWindowsLongPathsStatus(async () => ({
      stdout: [
        "HKEY_LOCAL_MACHINE\\SYSTEM\\CurrentControlSet\\Control\\FileSystem",
        "    LongPathsEnabled    REG_DWORD    0x0",
      ].join("\r\n"),
      stderr: "",
    }));

    expect(status).toBe("disabled");
  });

  test("inspects deps and templates from the config-root runtime directory instead of workdir", async () => {
    const installRoot = await fs.mkdtemp(path.join(os.tmpdir(), "rayleabot-install-"));
    const workdir = await fs.mkdtemp(path.join(os.tmpdir(), "rayleabot-workdir-"));
    const configDir = path.join(installRoot, "config");
    const serverExecutablePath = path.join(installRoot, process.platform === "win32" ? "raylea-server.exe" : "raylea-server");
    await fs.mkdir(configDir, { recursive: true });
    await fs.mkdir(path.join(installRoot, ".deps"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "templates", "help.menu"), { recursive: true });
    await fs.writeFile(serverExecutablePath, "", "utf8");
    await fs.writeFile(path.join(configDir, "user.yaml"), "server:\n  host: 127.0.0.1\n  port: 8080\n", "utf8");
    await fs.writeFile(path.join(configDir, "default.yaml"), "server:\n  host: 127.0.0.1\n  port: 8080\n", "utf8");
    await fs.writeFile(path.join(installRoot, "templates", "help.menu", "template.json"), "{}", "utf8");

    const arch = os.arch() === "amd64" ? "x64" : os.arch();
    const platform = process.platform === "win32" ? `windows-${arch}` : process.platform === "darwin" ? `macos-${arch}` : `${process.platform}-${arch}`;
    await fs.writeFile(
      path.join(installRoot, ".deps", "manifest.json"),
      JSON.stringify({
        manifest_version: 2,
        resources: [
          {
            id: `chromium-${platform}`,
            platform,
            kind: "chromium",
            version: "147.0.7727.24",
            source: "https://example.invalid/chromium.zip",
            sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
            archive_format: "zip",
            entrypoints: { browser: ["chrome/chrome"] },
          },
          {
            id: `python-${platform}`,
            platform,
            kind: "python-runtime",
            version: "3.12.13",
            source: "https://example.invalid/python.tar.gz",
            sha256: "10b9fd9ba9441f246f2cb279c2c6e6b2f98e60ef7960c313fd2bbc7f0c1e6f5e",
            archive_format: "tar.gz",
            entrypoints: { python: ["python/bin/python"], pip: ["python/bin/pip"] },
          },
          {
            id: `node-${platform}`,
            platform,
            kind: "nodejs-runtime",
            version: "24.14.0",
            source: "https://example.invalid/node.zip",
            sha256: "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
            archive_format: "zip",
            entrypoints: { node: ["node/bin/node"], npm: ["node/bin/npm"] },
          },
        ],
      }),
      "utf8",
    );

    const settings: LauncherResolvedSettings = {
      installationRoot: installRoot,
      serverExecutablePath,
      configPath: path.join(configDir, "user.yaml"),
      workdir,
    };

    try {
      const inspection = await inspectEnvironmentFromNode(settings);

      expect(inspection.checks.some((item) => item.code === "deps.manifest" && item.severity === "ok")).toBe(true);
      expect(inspection.checks.some((item) => item.code === "render.templates" && item.severity === "ok")).toBe(true);
      expect(inspection.checks.some((item) => item.code === "runtime.python_managed_ready" && item.severity === "ok")).toBe(true);
    } finally {
      await fs.rm(installRoot, { recursive: true, force: true });
      await fs.rm(workdir, { recursive: true, force: true });
    }
  });

  test("distinguishes prepared, cached, and on-demand managed runtime states from the runtime root", async () => {
    const installRoot = await fs.mkdtemp(path.join(os.tmpdir(), "rayleabot-runtime-root-"));
    const workdir = await fs.mkdtemp(path.join(os.tmpdir(), "rayleabot-runtime-workdir-"));
    const configDir = path.join(installRoot, "config");
    const serverExecutablePath = path.join(installRoot, process.platform === "win32" ? "raylea-server.exe" : "raylea-server");
    await fs.mkdir(configDir, { recursive: true });
    await fs.mkdir(path.join(installRoot, ".deps"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "templates", "help.menu"), { recursive: true });
    await fs.mkdir(path.join(installRoot, "cache", "downloads", "runtime"), { recursive: true });
    await fs.writeFile(serverExecutablePath, "", "utf8");
    await fs.writeFile(path.join(configDir, "user.yaml"), "server:\n  host: 127.0.0.1\n  port: 8080\n", "utf8");
    await fs.writeFile(path.join(configDir, "default.yaml"), "server:\n  host: 127.0.0.1\n  port: 8080\n", "utf8");
    await fs.writeFile(path.join(installRoot, "templates", "help.menu", "template.json"), "{}", "utf8");

    const arch = os.arch() === "amd64" ? "x64" : os.arch();
    const platform = process.platform === "win32" ? `windows-${arch}` : process.platform === "darwin" ? `macos-${arch}` : `${process.platform}-${arch}`;
    const chromiumId = `chromium-${platform}`;
    const pythonId = `python-${platform}`;
    const nodeId = `node-${platform}`;
    await fs.writeFile(
      path.join(installRoot, ".deps", "manifest.json"),
      JSON.stringify({
        manifest_version: 2,
        resources: [
          {
            id: chromiumId,
            platform,
            kind: "chromium",
            version: "147.0.7727.24",
            source: "https://example.invalid/chromium.zip",
            sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
            archive_format: "zip",
            entrypoints: { browser: ["chrome/chrome"] },
          },
          {
            id: pythonId,
            platform,
            kind: "python-runtime",
            version: "3.12.13",
            source: "https://example.invalid/python.tar.gz",
            sha256: "10b9fd9ba9441f246f2cb279c2c6e6b2f98e60ef7960c313fd2bbc7f0c1e6f5e",
            archive_format: "tar.gz",
            entrypoints: { python: ["python/bin/python"], pip: ["python/bin/pip"] },
          },
          {
            id: nodeId,
            platform,
            kind: "nodejs-runtime",
            version: "24.14.0",
            source: "https://example.invalid/node.zip",
            sha256: "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
            archive_format: "zip",
            entrypoints: { node: ["node/bin/node"], npm: ["node/bin/npm"] },
          },
        ],
      }),
      "utf8",
    );
    await fs.mkdir(path.join(installRoot, ".deps", "store", chromiumId, "147.0.7727.24", "chrome"), { recursive: true });
    await fs.writeFile(path.join(installRoot, ".deps", "store", chromiumId, "147.0.7727.24", "chrome", "chrome"), "", "utf8");
    await fs.writeFile(path.join(installRoot, "cache", "downloads", "runtime", `${pythonId}-3.12.13.tar.gz`), "", "utf8");

    const settings: LauncherResolvedSettings = {
      installationRoot: installRoot,
      serverExecutablePath,
      configPath: path.join(configDir, "user.yaml"),
      workdir,
    };

    try {
      const inspection = await inspectEnvironmentFromNode(settings);

      expect(inspection.checks.find((item) => item.code === "deps.chromium")?.summary).toBe("Chromium 资源已准备完成。");
      expect(inspection.checks.find((item) => item.code === "runtime.python_managed_ready")?.summary).toBe("受控 Python 运行时归档已缓存，可离线准备。");
      expect(inspection.checks.find((item) => item.code === "runtime.node_managed_ready")?.summary).toBe("受控 Node.js 运行时可按需准备。");
    } finally {
      await fs.rm(installRoot, { recursive: true, force: true });
      await fs.rm(workdir, { recursive: true, force: true });
    }
  });
});

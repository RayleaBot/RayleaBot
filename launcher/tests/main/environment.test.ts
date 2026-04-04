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
        manifest_version: 3,
        resources: [
          {
            platform: "windows-x64",
            kind: "chromium",
            sources: [{ url: "https://example.invalid/chromium.zip", kind: "upstream" }],
            sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
            archive_format: "zip",
            entrypoints: { browser: ["chrome-win64/chrome.exe"] },
          },
          {
            platform: "windows-x64",
            kind: "nodejs-runtime",
            sources: [{ url: "https://example.invalid/node.zip", kind: "upstream" }],
            sha256: "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
            archive_format: "zip",
            entrypoints: { node: ["node/node.exe"], npm: ["node/npm.cmd"] },
          },
          {
            platform: "windows-x64",
            kind: "python-runtime",
            sources: [{ url: "https://example.invalid/python.tar.gz", kind: "upstream" }],
            sha256: "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
            archive_format: "tar.gz",
            entrypoints: { python: ["python/python.exe"], pip: ["python/Scripts/pip.exe"] },
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
        manifest_version: 3,
        resources: [{ platform: "windows-x64", kind: "chromium", archive_format: "zip", sources: [{ url: "https://example.invalid/chromium.zip", kind: "upstream" }], sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e", entrypoints: { browser: ["chrome-win64/chrome.exe"] } }],
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
        manifest_version: 3,
        resources: [
          {
            id: "python-windows-x64",
            platform: "windows-x64",
            kind: "python-runtime",
            version: "3.12.13",
            sources: [{ url: "https://example.invalid/python.zip", kind: "upstream" }],
            sha256: "deadbeef",
            archive_format: "zip",
            entrypoints: { python: ["python/python.exe"], pip: ["python/Scripts/pip.exe"] },
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
        manifest_version: 3,
        resources: [
          {
            id: "chromium-windows-x64",
            platform: "windows-x64",
            kind: "chromium",
            version: "147.0.7727.24",
            sources: [{ url: "https://storage.googleapis.com/chrome-for-testing-public/147.0.7727.24/win64/chrome-win64.zip", kind: "upstream" }],
            sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
            archive_format: "zip",
            entrypoints: { browser: ["chrome-win64/chrome.exe"] },
          },
          {
            id: "python-windows-x64",
            platform: "windows-x64",
            kind: "python-runtime",
            version: "3.12.13",
            sources: [{ url: "http://example.invalid/python.tar.gz", kind: "upstream" }],
            sha256: "not-a-sha256",
            archive_format: "tar.gz",
            entrypoints: { python: ["python/python.exe"], pip: ["python/Scripts/pip.exe"] },
          },
          {
            id: "nodejs-windows-x64",
            platform: "windows-x64",
            kind: "nodejs-runtime",
            version: "24.14.0",
            sources: [{ url: "https://nodejs.org/download/release/v24.14.0/node-v24.14.0-win-x64.zip", kind: "upstream" }],
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

  test("does not treat contract-valid metadata as incomplete only because the source path contains TODO marker text", async () => {
    const inspection = await inspectLauncherEnvironment({
      serverExecutableExists: true,
      userConfigExists: true,
      defaultConfigExists: true,
      workdirWritable: true,
      depsManifestExists: true,
      depsManifestText: JSON.stringify({
        manifest_version: 3,
        resources: [
          {
            id: "chromium-windows-x64",
            platform: "windows-x64",
            kind: "chromium",
            version: "147.0.7727.24",
            sources: [{ url: "https://example.invalid/runtime/TODO(chromium).zip", kind: "upstream" }],
            sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
            archive_format: "zip",
            entrypoints: { browser: ["chrome-win64/chrome.exe"] },
          },
        ],
      }),
      templatesExist: true,
      templatesHaveFiles: true,
      platform: "windows-x64",
      longPaths: "enabled",
    });

    expect(inspection.checks.find((item) => item.code === "deps.chromium")?.severity).toBe("ok");
    expect(inspection.checks.find((item) => item.code === "deps.chromium")?.summary).toBe("Chromium 资源可按需准备。");
  });

  test("flags missing template directory for render resources", async () => {
    const inspection = await inspectLauncherEnvironment({
      serverExecutableExists: true,
      userConfigExists: true,
      defaultConfigExists: true,
      workdirWritable: true,
      depsManifestExists: true,
      depsManifestText: JSON.stringify({
        manifest_version: 3,
        resources: [
          {
            id: "chromium-windows-x64",
            platform: "windows-x64",
            kind: "chromium",
            version: "147.0.7727.24",
            sources: [{ url: "https://storage.googleapis.com/chrome-for-testing-public/147.0.7727.24/win64/chrome-win64.zip", kind: "upstream" }],
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
        manifest_version: 3,
        resources: [
          {
            id: `chromium-${platform}`,
            platform,
            kind: "chromium",
            version: "147.0.7727.24",
            sources: [{ url: "https://example.invalid/chromium.zip", kind: "upstream" }],
            sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
            archive_format: "zip",
            entrypoints: { browser: ["chrome/chrome"] },
          },
          {
            id: `python-${platform}`,
            platform,
            kind: "python-runtime",
            version: "3.12.13",
            sources: [{ url: "https://example.invalid/python.tar.gz", kind: "upstream" }],
            sha256: "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
            archive_format: "tar.gz",
            entrypoints: { python: ["python/bin/python"], pip: ["python/bin/pip"] },
          },
          {
            id: `node-${platform}`,
            platform,
            kind: "nodejs-runtime",
            version: "24.14.0",
            sources: [{ url: "https://example.invalid/node.zip", kind: "upstream" }],
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
        manifest_version: 3,
        resources: [
          {
            id: chromiumId,
            platform,
            kind: "chromium",
            version: "147.0.7727.24",
            sources: [{ url: "https://example.invalid/chromium.zip", kind: "upstream" }],
            sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
            archive_format: "zip",
            entrypoints: { browser: ["chrome/chrome"] },
          },
          {
            id: pythonId,
            platform,
            kind: "python-runtime",
            version: "3.12.13",
            sources: [{ url: "https://example.invalid/python.tar.gz", kind: "upstream" }],
            sha256: "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
            archive_format: "tar.gz",
            entrypoints: { python: ["python/bin/python"], pip: ["python/bin/pip"] },
          },
          {
            id: nodeId,
            platform,
            kind: "nodejs-runtime",
            version: "24.14.0",
            sources: [{ url: "https://example.invalid/node.zip", kind: "upstream" }],
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
      expect(inspection.checks.find((item) => item.code === "runtime.python_managed_ready")?.summary).toBe("Python 运行环境安装包已缓存，启动时可直接完成准备。");
      expect(inspection.checks.find((item) => item.code === "runtime.node_managed_ready")?.summary).toBe("Node.js 与 npm 环境已纳入启动流程。");
    } finally {
      await fs.rm(installRoot, { recursive: true, force: true });
      await fs.rm(workdir, { recursive: true, force: true });
    }
  });
});

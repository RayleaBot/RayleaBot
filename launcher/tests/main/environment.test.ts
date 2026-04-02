import { describe, expect, test } from "vitest";
import {
  detectWindowsLongPathsStatus,
  inspectLauncherEnvironment,
} from "@main/services/environment";

describe("inspectLauncherEnvironment", () => {
  test("reports bootstrap_available when user config is missing but default template exists", async () => {
    const inspection = await inspectLauncherEnvironment({
      serverExecutableExists: true,
      userConfigExists: false,
      defaultConfigExists: true,
      workdirWritable: true,
      depsManifestExists: true,
      depsManifestText: JSON.stringify({
        resources: [
          { platform: "windows-x64", kind: "chromium" },
          { platform: "windows-x64", kind: "nodejs-runtime" },
          { platform: "windows-x64", kind: "python-runtime" },
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
        resources: [{ platform: "windows-x64", kind: "chromium" }],
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
        resources: [
          {
            id: "python-windows-x64",
            platform: "windows-x64",
            kind: "python-runtime",
            version: "3.12.13",
            source: "https://example.invalid/python.zip",
            sha256: "deadbeef",
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
        resources: [
          {
            id: "chromium-windows-x64",
            platform: "windows-x64",
            kind: "chromium",
            version: "147.0.7727.24",
            source: "https://storage.googleapis.com/chrome-for-testing-public/147.0.7727.24/win64/chrome-win64.zip",
            sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
          },
          {
            id: "python-windows-x64",
            platform: "windows-x64",
            kind: "python-runtime",
            version: "3.12.13",
            source: "TODO(v0.1-phase0)",
            sha256: "TODO(v0.1-phase0)",
          },
          {
            id: "nodejs-windows-x64",
            platform: "windows-x64",
            kind: "nodejs-runtime",
            version: "24.14.0",
            source: "https://nodejs.org/download/release/v24.14.0/node-v24.14.0-win-x64.zip",
            sha256: "deadbeef",
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
        resources: [
          {
            id: "chromium-windows-x64",
            platform: "windows-x64",
            kind: "chromium",
            version: "147.0.7727.24",
            source: "https://storage.googleapis.com/chrome-for-testing-public/147.0.7727.24/win64/chrome-win64.zip",
            sha256: "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
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
});

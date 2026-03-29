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

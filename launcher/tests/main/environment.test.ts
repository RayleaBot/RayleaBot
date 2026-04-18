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

  test("inspects only local preflight inputs from the installation root", async () => {
    const installRoot = await fs.mkdtemp(path.join(os.tmpdir(), "rayleabot-install-"));
    const workdir = await fs.mkdtemp(path.join(os.tmpdir(), "rayleabot-workdir-"));
    const configDir = path.join(installRoot, "config");
    const serverExecutablePath = path.join(installRoot, process.platform === "win32" ? "raylea-server.exe" : "raylea-server");
    await fs.mkdir(configDir, { recursive: true });
    await fs.writeFile(serverExecutablePath, "", "utf8");
    await fs.writeFile(path.join(configDir, "default.yaml"), "server:\n  host: 127.0.0.1\n  port: 8080\n", "utf8");

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
      expect(inspection.checks.some((item) => /^(deps|runtime|render|os)\./.test(item.code))).toBe(false);
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
      expect(inspection.checks.some((item) => /^(deps|runtime|render|os)\./.test(item.code))).toBe(false);
      expect(inspection.advisoryChecks).toEqual([]);
    } finally {
      await fs.rm(installRoot, { recursive: true, force: true });
    }
  });
});

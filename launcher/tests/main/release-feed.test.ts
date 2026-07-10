import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, test, vi } from "vitest";
import {
  LauncherReleaseFeedClient,
  type LauncherUpdaterRunner,
} from "@main/services/release-feed";

const tempRoots: string[] = [];

async function createTempDir(label: string) {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), `raylea-update-${label}-`));
  tempRoots.push(tempRoot);
  return tempRoot;
}

async function writeBuildInfo(basePath: string, version = "1.0.0") {
  await fs.writeFile(
    path.join(basePath, "build_info.json"),
    JSON.stringify({
      version,
      git_commit: "0123456789abcdef0123456789abcdef01234567",
      artifact_id: "windows-x64-full",
      built_at: "2026-07-10T00:00:00Z",
      update_protocol_version: 2,
    }),
    "utf8",
  );
}

function checkResult(overrides: Record<string, unknown> = {}) {
  return {
    status: "update_available",
    current_version: "1.0.0",
    available_version: "1.1.0",
    update_mode: "automatic",
    release_page_url: "https://github.com/RayleaBot/RayleaBot/releases/tag/v1.1.0",
    automatic_install_supported: true,
    manifest_path: "",
    signature_path: "",
    manifest_sha256: "a".repeat(64),
    artifact: {
      artifact_id: "windows-x64-full",
      file_name: "RayleaBot-v1.1.0-windows-x64-full.zip",
      archive_size_bytes: 1024,
      update_mode: "automatic",
    },
    ...overrides,
  };
}

class FakeUpdaterRunner implements LauncherUpdaterRunner {
  run = vi.fn(async () => ({ stdout: JSON.stringify(checkResult()), stderr: "" }));
  spawnDetached = vi.fn(async () => undefined);
}

afterEach(async () => {
  await Promise.all(tempRoots.splice(0).map((target) => fs.rm(target, { recursive: true, force: true })));
});

describe("LauncherReleaseFeedClient", () => {
  test("requires a manually installed v2 trust baseline", async () => {
    const basePath = await createTempDir("baseline");
    const runner = new FakeUpdaterRunner();
    const client = new LauncherReleaseFeedClient(basePath, { platform: "win32", runner });

    const snapshot = await client.getSnapshot();

    expect(snapshot.status).toBe("disabled");
    expect(snapshot.detail).toContain("手动安装");
    expect(runner.run).not.toHaveBeenCalled();
  });

  test("delegates trust decisions to the compiled Go updater", async () => {
    const basePath = await createTempDir("check");
    await writeBuildInfo(basePath);
    const updaterPath = path.join(basePath, "raylea-updater.exe");
    await fs.writeFile(updaterPath, "trusted helper");
    const runner = new FakeUpdaterRunner();
    const client = new LauncherReleaseFeedClient(basePath, { platform: "win32", runner, updaterPath });

    const snapshot = await client.getSnapshot({ force: true });

    expect(runner.run).toHaveBeenCalledWith(updaterPath, ["check", "--install-root", basePath, "--json"]);
    expect(snapshot.status).toBe("update_available");
    expect(snapshot.canDownload).toBe(true);
    expect(snapshot.canInstall).toBe(false);
  });

  test("keeps guided releases out of the automatic install path", async () => {
    const basePath = await createTempDir("guided");
    await writeBuildInfo(basePath);
    const updaterPath = path.join(basePath, "raylea-updater.exe");
    await fs.writeFile(updaterPath, "trusted helper");
    const runner = new FakeUpdaterRunner();
    runner.run.mockResolvedValue({
      stdout: JSON.stringify(checkResult({
        update_mode: "guided",
        automatic_install_supported: false,
        artifact: { ...checkResult().artifact, update_mode: "guided" },
      })),
      stderr: "",
    });
    const client = new LauncherReleaseFeedClient(basePath, { platform: "win32", runner, updaterPath });

    const snapshot = await client.getSnapshot({ force: true });

    expect(snapshot.status).toBe("update_available");
    expect(snapshot.canDownload).toBe(false);
    expect(snapshot.detail).toContain("引导更新");
  });

  test("uses an external sibling helper for the transaction and never PowerShell", async () => {
    const parent = await createTempDir("transaction");
    const basePath = path.join(parent, "RayleaBot");
    await fs.mkdir(basePath);
    await writeBuildInfo(basePath);
    const updaterPath = path.join(basePath, "raylea-updater.exe");
    const manifestPath = path.join(basePath, "cache", "downloads", "updates", "release_manifest.v2.json");
    const signaturePath = path.join(basePath, "cache", "downloads", "updates", "release_manifest.v2.sig.json");
    const artifactPath = path.join(basePath, "cache", "downloads", "updates", "RayleaBot-v1.1.0-windows-x64-full.zip");
    await fs.mkdir(path.dirname(manifestPath), { recursive: true });
    await Promise.all([
      fs.writeFile(updaterPath, "trusted helper"),
      fs.writeFile(manifestPath, "manifest"),
      fs.writeFile(signaturePath, "signature"),
      fs.writeFile(artifactPath, "artifact"),
    ]);
    const runner = new FakeUpdaterRunner();
    runner.run
      .mockResolvedValueOnce({ stdout: JSON.stringify(checkResult({ manifest_path: manifestPath, signature_path: signaturePath })), stderr: "" })
      .mockResolvedValueOnce({ stdout: JSON.stringify({ ...checkResult({ manifest_path: manifestPath, signature_path: signaturePath }), artifact_path: artifactPath }), stderr: "" });
    const client = new LauncherReleaseFeedClient(basePath, { platform: "win32", runner, updaterPath });

    await client.getSnapshot({ force: true });
    expect((await client.downloadUpdate()).status).toBe("ready_to_install");
    const installing = await client.installDownloadedUpdate(4242, true, {
      installationRoot: basePath,
      serverExecutablePath: path.join(basePath, "raylea-server.exe"),
      configPath: path.join(basePath, "config", "user.yaml"),
      workdir: basePath,
    });

    expect(installing.status).toBe("installing");
    expect(runner.spawnDetached).toHaveBeenCalledTimes(1);
    const [externalHelper, args] = runner.spawnDetached.mock.calls[0]!;
    expect(path.dirname(externalHelper)).toBe(args[args.indexOf("--transaction-root") + 1]);
    expect(path.dirname(path.dirname(externalHelper))).toBe(parent);
    expect(path.basename(path.dirname(externalHelper))).toMatch(/^\.rayleabot-update-/);
    expect(args).toContain("--service-was-running=true");
    expect([externalHelper, ...args].join(" ").toLowerCase()).not.toContain("powershell");
  });

  test("does not expose helper stderr paths to the renderer", async () => {
    const basePath = await createTempDir("failure");
    await writeBuildInfo(basePath);
    const updaterPath = path.join(basePath, "raylea-updater.exe");
    await fs.writeFile(updaterPath, "trusted helper");
    const runner = new FakeUpdaterRunner();
    runner.run.mockRejectedValue(new Error("更新助手拒绝了操作（release.signature_invalid）。"));
    const client = new LauncherReleaseFeedClient(basePath, { platform: "win32", runner, updaterPath });

    const snapshot = await client.getSnapshot({ force: true });

    expect(snapshot.status).toBe("failed");
    expect(snapshot.detail).toContain("release.signature_invalid");
    expect(snapshot.detail).not.toContain(basePath);
  });
});

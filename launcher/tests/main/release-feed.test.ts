import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, test, vi } from "vitest";
import { LauncherReleaseFeedClient } from "@main/services/release-feed";

const tempRoots: string[] = [];
const archiveFileName = "RayleaBot-1.2.0-windows-x64-full.zip";
const archiveContent = Buffer.from("fake rayleabot windows archive");
const archiveSha256 = "8637f38c0a98fab648e1fcacac7746563ab44e2065861101a42a68925768f6ce";

async function createTempDir(label: string) {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), `raylea-release-${label}-`));
  tempRoots.push(tempRoot);
  return tempRoot;
}

async function writeBuildInfo(basePath: string, version = "1.0.0") {
  await fs.writeFile(
    path.join(basePath, "build_info.json"),
    JSON.stringify({
      version,
      git_commit: "abcdef1",
      artifact_id: "windows-x64-full",
      built_at: "2026-06-26T00:00:00Z",
      release_notes_ref: `https://github.com/rayleabot/rayleabot/releases/tag/v${version}`,
    }),
    "utf8",
  );
}

function releasePayload(overrides: Record<string, unknown> = {}) {
  return {
    tag_name: "v1.2.0",
    html_url: "https://github.com/rayleabot/rayleabot/releases/tag/v1.2.0",
    assets: [
      {
        name: "release_manifest.json",
        browser_download_url: "https://github.com/rayleabot/rayleabot/releases/download/v1.2.0/release_manifest.json",
      },
      {
        name: archiveFileName,
        browser_download_url: "https://github.com/rayleabot/rayleabot/releases/download/v1.2.0/RayleaBot.zip",
      },
    ],
    ...overrides,
  };
}

function manifestPayload(overrides: Record<string, unknown> = {}) {
  return {
    version: "1.2.0",
    git_commit: "abcdef1",
    built_at: "2026-06-26T00:00:00Z",
    config_schema_version: "1",
    db_schema_version: "1",
    plugin_protocol_version: "1",
    release_notes_ref: "https://github.com/rayleabot/rayleabot/releases/tag/v1.2.0",
    artifacts: [
      {
        artifact_id: "windows-x64-full",
        file_name: archiveFileName,
        platform: "windows-x64",
        sha256: archiveSha256,
        size: archiveContent.byteLength,
        support_level: "first_class",
        deps_manifest_sha256: "0".repeat(64),
        smoke_profile: "windows_full_smoke",
      },
    ],
    ...overrides,
  };
}

function jsonResponse(payload: unknown) {
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    json: async () => payload,
    text: async () => JSON.stringify(payload),
  } satisfies Partial<Response> as Response;
}

function bufferResponse(buffer: Buffer) {
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    body: new Response(buffer).body,
    arrayBuffer: async () => buffer.buffer.slice(buffer.byteOffset, buffer.byteOffset + buffer.byteLength),
  } satisfies Partial<Response> as Response;
}

afterEach(async () => {
  vi.unstubAllGlobals();
  await Promise.all(
    tempRoots.splice(0).map(async (target) => {
      await fs.rm(target, { recursive: true, force: true });
    }),
  );
});

describe("LauncherReleaseFeedClient", () => {
  test("disables updates for development runs without build metadata and does not fetch", async () => {
    const basePath = await createTempDir("development");
    const fetchLike = vi.fn();

    const client = new LauncherReleaseFeedClient(basePath, { fetchLike, platform: "win32" });
    const snapshot = await client.getSnapshot();

    expect(snapshot.status).toBe("disabled");
    expect(snapshot.summary).toBe("开发版本不支持更新");
    expect(snapshot.canCheck).toBe(false);
    expect(fetchLike).not.toHaveBeenCalled();
  });

  test("preserves the packaged version when the remote release feed is unavailable", async () => {
    const basePath = await createTempDir("fetch-failure");
    await writeBuildInfo(basePath, "0.1.0");

    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("fetch failed");
      }),
    );

    const client = new LauncherReleaseFeedClient(basePath);
    const snapshot = await client.getSnapshot();

    expect(snapshot.status).toBe("error");
    expect(snapshot.currentVersion).toBe("0.1.0");
    expect(snapshot.summary).toBe("暂时无法连接版本源。");
  });

  test("attaches a timeout signal to GitHub release requests", async () => {
    const basePath = await createTempDir("timeout-signal");
    await writeBuildInfo(basePath, "1.2.0");

    let receivedSignal: AbortSignal | undefined;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        receivedSignal = init?.signal as AbortSignal | undefined;
        return String(input).endsWith("release_manifest.json")
          ? jsonResponse(manifestPayload({ version: "1.2.0" }))
          : jsonResponse(releasePayload());
      }),
    );

    const client = new LauncherReleaseFeedClient(basePath);
    const snapshot = await client.getSnapshot();

    expect(snapshot.status).toBe("up_to_date");
    expect(receivedSignal).toBeDefined();
    expect(receivedSignal?.aborted).toBe(false);
  });

  test("treats build metadata as semver-equivalent to the packaged version", async () => {
    const basePath = await createTempDir("build-metadata");
    await writeBuildInfo(basePath, "1.0.0");

    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) =>
        String(input).endsWith("release_manifest.json")
          ? jsonResponse(manifestPayload({ version: "1.0.0+build.1" }))
          : jsonResponse(releasePayload({ html_url: "https://github.com/rayleabot/rayleabot/releases/tag/v1.0.0+build.1" })),
      ),
    );

    const client = new LauncherReleaseFeedClient(basePath);
    const snapshot = await client.getSnapshot();

    expect(snapshot.status).toBe("up_to_date");
    expect(snapshot.latestVersion).toBe("1.0.0");
  });

  test("treats prerelease builds as older than the final release", async () => {
    const basePath = await createTempDir("prerelease");
    await writeBuildInfo(basePath, "1.0.0");

    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) =>
        String(input).endsWith("release_manifest.json")
          ? jsonResponse(manifestPayload({ version: "1.0.0-rc.1+build.2" }))
          : jsonResponse(releasePayload({ html_url: "https://github.com/rayleabot/rayleabot/releases/tag/v1.0.0-rc.1" })),
      ),
    );

    const client = new LauncherReleaseFeedClient(basePath);
    const snapshot = await client.getSnapshot();

    expect(snapshot.status).toBe("up_to_date");
    expect(snapshot.latestVersion).toBe("1.0.0");
  });

  test("reports an available Windows full package from release_manifest.json", async () => {
    const basePath = await createTempDir("available");
    await writeBuildInfo(basePath, "1.0.0");
    const fetchLike = vi.fn(async (input: RequestInfo | URL) =>
      String(input).endsWith("release_manifest.json")
        ? jsonResponse(manifestPayload())
        : jsonResponse(releasePayload()),
    );

    const client = new LauncherReleaseFeedClient(basePath, { fetchLike, platform: "win32" });
    const snapshot = await client.getSnapshot();

    expect(snapshot.status).toBe("update_available");
    expect(snapshot.latestVersion).toBe("1.2.0");
    expect(snapshot.artifactFileName).toBe(archiveFileName);
    expect(snapshot.totalBytes).toBe(archiveContent.byteLength);
    expect(snapshot.canDownload).toBe(true);
  });

  test("reports readable errors for missing manifest, artifact, and invalid versions", async () => {
    const missingManifestBase = await createTempDir("missing-manifest");
    await writeBuildInfo(missingManifestBase, "1.0.0");
    const missingManifest = new LauncherReleaseFeedClient(missingManifestBase, {
      fetchLike: vi.fn(async () => jsonResponse(releasePayload({ assets: [] }))),
      platform: "win32",
    });
    expect((await missingManifest.getSnapshot()).detail).toContain("release_manifest.json");

    const missingArtifactBase = await createTempDir("missing-artifact");
    await writeBuildInfo(missingArtifactBase, "1.0.0");
    const missingArtifact = new LauncherReleaseFeedClient(missingArtifactBase, {
      fetchLike: vi.fn(async (input: RequestInfo | URL) =>
        String(input).endsWith("release_manifest.json")
          ? jsonResponse(manifestPayload({ artifacts: [] }))
          : jsonResponse(releasePayload()),
      ),
      platform: "win32",
    });
    expect((await missingArtifact.getSnapshot()).detail).toContain("windows-x64-full");

    const invalidVersionBase = await createTempDir("invalid-version");
    await writeBuildInfo(invalidVersionBase, "1.0.0");
    const invalidVersion = new LauncherReleaseFeedClient(invalidVersionBase, {
      fetchLike: vi.fn(async (input: RequestInfo | URL) =>
        String(input).endsWith("release_manifest.json")
          ? jsonResponse(manifestPayload({ version: "latest" }))
          : jsonResponse(releasePayload()),
      ),
      platform: "win32",
    });
    const snapshot = await invalidVersion.getSnapshot();
    expect(snapshot.status).toBe("error");
    expect(snapshot.summary).toBe("发布源返回的版本号无法比较。");
  });

  test("downloads the matched archive and verifies size and SHA256", async () => {
    const basePath = await createTempDir("download");
    await writeBuildInfo(basePath, "1.0.0");
    const progress: number[] = [];
    const fetchLike = vi.fn(async (input: RequestInfo | URL) => {
      const value = String(input);
      if (value.endsWith("release_manifest.json")) {
        return jsonResponse(manifestPayload());
      }
      if (value.endsWith("RayleaBot.zip")) {
        return bufferResponse(archiveContent);
      }
      return jsonResponse(releasePayload());
    });
    const client = new LauncherReleaseFeedClient(basePath, { fetchLike, platform: "win32" });

    await client.getSnapshot();
    const snapshot = await client.downloadUpdate((next) => {
      if (next.downloadProgress !== null) {
        progress.push(next.downloadProgress);
      }
    });

    expect(snapshot.status).toBe("downloaded");
    expect(snapshot.canInstall).toBe(true);
    expect(progress.at(-1)).toBe(1);
    await expect(fs.stat(path.join(basePath, "cache", "downloads", "updates", archiveFileName))).resolves.toBeDefined();
  });

  test("keeps the update downloadable when SHA256 verification fails", async () => {
    const basePath = await createTempDir("sha-mismatch");
    await writeBuildInfo(basePath, "1.0.0");
    const fetchLike = vi.fn(async (input: RequestInfo | URL) => {
      const value = String(input);
      if (value.endsWith("release_manifest.json")) {
        return jsonResponse(manifestPayload({ artifacts: [{ ...manifestPayload().artifacts[0], sha256: "f".repeat(64) }] }));
      }
      if (value.endsWith("RayleaBot.zip")) {
        return bufferResponse(archiveContent);
      }
      return jsonResponse(releasePayload());
    });
    const client = new LauncherReleaseFeedClient(basePath, { fetchLike, platform: "win32" });

    await client.getSnapshot();
    const snapshot = await client.downloadUpdate();

    expect(snapshot.status).toBe("error");
    expect(snapshot.detail).toBe("下载包校验失败。");
    expect(snapshot.canDownload).toBe(true);
  });
});

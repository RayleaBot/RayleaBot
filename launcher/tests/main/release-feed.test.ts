import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, test, vi } from "vitest";
import { LauncherReleaseFeedClient } from "@main/services/release-feed";

const tempRoots: string[] = [];

async function createTempDir(label: string) {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), `raylea-release-${label}-`));
  tempRoots.push(tempRoot);
  return tempRoot;
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
  test("preserves the packaged version when the remote release feed is unavailable", async () => {
    const basePath = await createTempDir("fetch-failure");
    await fs.writeFile(
      path.join(basePath, "build_info.json"),
      JSON.stringify({
        version: "0.1.0",
        release_notes_ref: "https://github.com/rayleabot/rayleabot/releases/tag/v0.1.0",
      }),
      "utf8",
    );

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
    await fs.writeFile(
      path.join(basePath, "build_info.json"),
      JSON.stringify({
        version: "0.1.0",
        release_notes_ref: "https://github.com/rayleabot/rayleabot/releases/tag/v0.1.0",
      }),
      "utf8",
    );

    let receivedSignal: AbortSignal | undefined;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
        receivedSignal = init?.signal as AbortSignal | undefined;
        return {
          ok: true,
          json: async () => ({ tag_name: "v0.1.0", html_url: "https://github.com/rayleabot/rayleabot/releases/tag/v0.1.0" }),
          text: async () => "",
        } satisfies Partial<Response> as Response;
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
    await fs.writeFile(
      path.join(basePath, "build_info.json"),
      JSON.stringify({
        version: "1.0.0",
        release_notes_ref: "https://github.com/rayleabot/rayleabot/releases/tag/v1.0.0",
      }),
      "utf8",
    );

    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        ok: true,
        json: async () => ({
          tag_name: "v1.0.0+build.1",
          html_url: "https://github.com/rayleabot/rayleabot/releases/tag/v1.0.0+build.1",
        }),
        text: async () => "",
      }) satisfies Partial<Response> as Response),
    );

    const client = new LauncherReleaseFeedClient(basePath);
    const snapshot = await client.getSnapshot();

    expect(snapshot.status).toBe("up_to_date");
    expect(snapshot.latestVersion).toBe("1.0.0");
  });

  test("treats prerelease builds as older than the final release", async () => {
    const basePath = await createTempDir("prerelease");
    await fs.writeFile(
      path.join(basePath, "build_info.json"),
      JSON.stringify({
        version: "1.0.0",
        release_notes_ref: "https://github.com/rayleabot/rayleabot/releases/tag/v1.0.0",
      }),
      "utf8",
    );

    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        ok: true,
        json: async () => ({
          tag_name: "v1.0.0-rc.1+build.2",
          html_url: "https://github.com/rayleabot/rayleabot/releases/tag/v1.0.0-rc.1",
        }),
        text: async () => "",
      }) satisfies Partial<Response> as Response),
    );

    const client = new LauncherReleaseFeedClient(basePath);
    const snapshot = await client.getSnapshot();

    expect(snapshot.status).toBe("up_to_date");
    expect(snapshot.latestVersion).toBe("1.0.0");
  });
});

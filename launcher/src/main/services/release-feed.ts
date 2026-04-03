import fs from "node:fs/promises";
import path from "node:path";
import { compare as compareSemver, valid as validSemver } from "semver";
import { createReleaseUnavailable } from "../../shared/launcher-copy";

interface LauncherReleaseFeedClientOptions {
  fetchLike?: typeof fetch;
  requestTimeoutMs?: number;
}

function normalizeSemver(value: string) {
  const trimmed = value.trim();
  return validSemver(trimmed) ?? validSemver(trimmed.replace(/^[vV]/, ""));
}

function resolveRepositoryUrl(releaseNotesRef: string) {
  try {
    const url = new URL(releaseNotesRef);
    if (url.hostname.toLowerCase() !== "github.com") {
      return "";
    }
    const [owner, repo] = url.pathname.split("/").filter(Boolean);
    return owner && repo ? `https://github.com/${owner}/${repo}` : "";
  } catch {
    return "";
  }
}

export class LauncherReleaseFeedClient {
  private cachedAt = 0;
  private cached = createReleaseUnavailable();
  private readonly fetchLike: typeof fetch;
  private readonly requestTimeoutMs: number;

  constructor(private readonly basePath: string, options: LauncherReleaseFeedClientOptions = {}) {
    this.fetchLike = options.fetchLike ?? fetch;
    this.requestTimeoutMs = options.requestTimeoutMs ?? 5000;
  }

  async getSnapshot() {
    if (Date.now() - this.cachedAt < 60 * 60 * 1000) {
      return this.cached;
    }
    this.cached = await this.loadSnapshot();
    this.cachedAt = Date.now();
    return this.cached;
  }

  private async loadSnapshot() {
    const buildInfoPath = path.join(this.basePath, "build_info.json");
    let currentVersion = "";
    let releasePageUrl = "";
    try {
      const payload = JSON.parse(await fs.readFile(buildInfoPath, "utf8")) as Record<string, string>;
      currentVersion = payload.version ?? "";
      const releaseNotesRef = payload.release_notes_ref ?? "";
      releasePageUrl = releaseNotesRef;

      if (!currentVersion) {
        return createReleaseUnavailable("build_info.json 未声明当前包版本。");
      }

      const repositoryUrl = resolveRepositoryUrl(releaseNotesRef);
      if (!repositoryUrl) {
        return createReleaseUnavailable("当前包元数据未暴露可用的 GitHub 发布页。");
      }

      const latestReleaseResponse = await this.fetchLike(
        repositoryUrl.replace("https://github.com/", "https://api.github.com/repos/") + "/releases/latest",
        {
          headers: { Accept: "application/vnd.github+json", "User-Agent": "RayleaLauncher/0.1.0" },
          signal: AbortSignal.timeout(this.requestTimeoutMs),
        },
      );

      if (!latestReleaseResponse.ok) {
        throw new Error(`${latestReleaseResponse.status} ${latestReleaseResponse.statusText}`);
      }

      const latestPayload = (await latestReleaseResponse.json()) as Record<string, unknown>;
      let latestVersion = String(latestPayload.tag_name ?? currentVersion);
      releasePageUrl = String(latestPayload.html_url ?? releaseNotesRef);
      const assets = Array.isArray(latestPayload.assets) ? latestPayload.assets : [];
      for (const asset of assets as Array<Record<string, unknown>>) {
        if (asset.name !== "release_manifest.json" || typeof asset.browser_download_url !== "string") {
          continue;
        }
        const manifestResponse = await this.fetchLike(asset.browser_download_url, {
          headers: { Accept: "application/json", "User-Agent": "RayleaLauncher/0.1.0" },
          signal: AbortSignal.timeout(this.requestTimeoutMs),
        });
        if (!manifestResponse.ok) {
          break;
        }
        const manifest = (await manifestResponse.json()) as Record<string, unknown>;
        latestVersion = String(manifest.version ?? latestVersion);
        releasePageUrl = String(manifest.release_notes_ref ?? releasePageUrl);
        break;
      }

      const current = normalizeSemver(currentVersion);
      const latest = normalizeSemver(latestVersion);
      if (!current || !latest) {
        return createReleaseUnavailable("发布源返回的版本号无法与当前打包版本比较。");
      }

      if (compareSemver(latest, current) > 0) {
        return {
          status: "update_available",
          currentVersion,
          latestVersion: latest,
          summary: `发现新版本：${current} -> ${latest}。`,
          detail: "打开发布页即可查看已发布包的元数据和版本说明。",
          releasePageUrl,
          updateAvailable: true,
        };
      }

      return {
        status: "up_to_date",
        currentVersion,
        latestVersion: current,
        summary: `当前版本 ${current} 已是最新。`,
        detail: "",
        releasePageUrl,
        updateAvailable: false,
      };
    } catch (error) {
      const detail = error instanceof Error ? error.message : "版本源不可用";
      return {
        ...createReleaseUnavailable(detail),
        status: "error",
        currentVersion,
        summary: "暂时无法连接版本源。",
        releasePageUrl,
      };
    }
  }
}

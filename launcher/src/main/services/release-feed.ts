import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { compare as compareSemver, valid as validSemver } from "semver";
import type { ReleaseCheckSnapshot } from "../../shared/launcher-models";
import { createReleaseDisabled, createReleaseUnavailable } from "../../shared/launcher-copy";

interface LauncherReleaseFeedClientOptions {
  cacheTtlMs?: number;
  fetchLike?: typeof fetch;
  platform?: NodeJS.Platform;
  requestTimeoutMs?: number;
  targetArtifactId?: string;
}

interface BuildInfo {
  version: string;
  artifactId: string;
  releaseNotesRef: string;
}

interface ReleaseArtifactCandidate {
  artifactId: string;
  downloadUrl: string;
  fileName: string;
  releasePageUrl: string;
  sha256: string;
  size: number;
  version: string;
}

const DEFAULT_CACHE_TTL_MS = 6 * 60 * 60 * 1000;
const DEFAULT_TARGET_ARTIFACT_ID = "windows-x64-full";
const PRESERVED_TOP_LEVEL_DIRS = ["cache", "data", "logs"];

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

function createSnapshot(input: Partial<ReleaseCheckSnapshot>): ReleaseCheckSnapshot {
  const status = input.status ?? "unavailable";
  const canCheck = Boolean(input.canCheck ?? (
    status !== "disabled"
    && status !== "unavailable"
    && status !== "checking"
    && status !== "downloading"
    && status !== "installing"
    && Boolean(input.currentVersion)
  ));

  return {
    status,
    currentVersion: input.currentVersion ?? "",
    latestVersion: input.latestVersion ?? "",
    summary: input.summary ?? "版本信息不可用",
    detail: input.detail ?? "",
    releasePageUrl: input.releasePageUrl ?? "",
    updateAvailable: input.updateAvailable ?? false,
    downloadProgress: input.downloadProgress ?? null,
    downloadedBytes: input.downloadedBytes ?? null,
    totalBytes: input.totalBytes ?? null,
    artifactFileName: input.artifactFileName ?? "",
    canCheck,
    canDownload: input.canDownload ?? status === "update_available",
    canInstall: input.canInstall ?? status === "downloaded",
  };
}

function updateDisabled(detail: string): ReleaseCheckSnapshot {
  return createSnapshot(createReleaseDisabled(detail));
}

function updateUnavailable(detail: string): ReleaseCheckSnapshot {
  return createSnapshot(createReleaseUnavailable(detail));
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function numberValue(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function psSingleQuoted(value: string) {
  return `'${value.replace(/'/g, "''")}'`;
}

async function sha256File(filePath: string) {
  const hash = createHash("sha256");
  const handle = await fs.open(filePath, "r");
  try {
    const stream = handle.createReadStream();
    for await (const chunk of stream) {
      hash.update(chunk);
    }
  } finally {
    await handle.close();
  }
  return hash.digest("hex");
}

async function ensureDirectory(directory: string) {
  await fs.mkdir(directory, { recursive: true });
}

export class LauncherReleaseFeedClient {
  private cachedAt = 0;
  private cached: ReleaseCheckSnapshot = updateUnavailable("尚未检查版本。");
  private downloadedArchivePath = "";
  private readonly cacheTtlMs: number;
  private readonly fetchLike: typeof fetch;
  private readonly platform: NodeJS.Platform;
  private readonly requestTimeoutMs: number;
  private readonly targetArtifactId: string;
  private updateCandidate: ReleaseArtifactCandidate | null = null;

  constructor(private readonly basePath: string, options: LauncherReleaseFeedClientOptions = {}) {
    this.cacheTtlMs = options.cacheTtlMs ?? DEFAULT_CACHE_TTL_MS;
    this.fetchLike = options.fetchLike ?? fetch;
    this.platform = options.platform ?? process.platform;
    this.requestTimeoutMs = options.requestTimeoutMs ?? 5000;
    this.targetArtifactId = options.targetArtifactId ?? DEFAULT_TARGET_ARTIFACT_ID;
  }

  async getSnapshot(options: { force?: boolean } = {}) {
    if (!options.force && Date.now() - this.cachedAt < this.cacheTtlMs) {
      return this.cached;
    }
    this.cached = await this.loadSnapshot();
    this.cachedAt = Date.now();
    return this.cached;
  }

  async downloadUpdate(onProgress?: (snapshot: ReleaseCheckSnapshot) => void | Promise<void>) {
    if (!this.updateCandidate) {
      const refreshed = await this.getSnapshot({ force: true });
      if (refreshed.status !== "update_available" || !this.updateCandidate) {
        return refreshed;
      }
    }

    const candidate = this.updateCandidate;
    const updateRoot = this.updateRoot();
    await ensureDirectory(updateRoot);
    const archivePath = path.join(updateRoot, candidate.fileName);
    const partialPath = `${archivePath}.download`;
    await fs.rm(partialPath, { force: true });

    const totalBytes = candidate.size;
    let downloadedBytes = 0;
    await onProgress?.(this.downloadingSnapshot(candidate, downloadedBytes, totalBytes));

    try {
      const response = await this.fetchLike(candidate.downloadUrl, {
        headers: { Accept: "application/octet-stream", "User-Agent": `RayleaLauncher/${candidate.version}` },
        signal: AbortSignal.timeout(this.requestTimeoutMs),
      });
      if (!response.ok) {
        throw new Error(`${response.status} ${response.statusText}`);
      }

      if (response.body) {
        const handle = await fs.open(partialPath, "w");
        try {
          const reader = response.body.getReader();
          while (true) {
            const next = await reader.read();
            if (next.done) {
              break;
            }
            downloadedBytes += next.value.byteLength;
            await handle.write(next.value);
            await onProgress?.(this.downloadingSnapshot(candidate, downloadedBytes, totalBytes));
          }
        } finally {
          await handle.close();
        }
      } else {
        const buffer = Buffer.from(await response.arrayBuffer());
        downloadedBytes = buffer.byteLength;
        await fs.writeFile(partialPath, buffer);
        await onProgress?.(this.downloadingSnapshot(candidate, downloadedBytes, totalBytes));
      }

      const stat = await fs.stat(partialPath);
      if (stat.size !== candidate.size) {
        throw new Error(`下载大小不一致：期望 ${candidate.size} 字节，实际 ${stat.size} 字节。`);
      }
      const digest = await sha256File(partialPath);
      if (digest.toLowerCase() !== candidate.sha256.toLowerCase()) {
        throw new Error("下载包校验失败。");
      }

      await fs.rm(archivePath, { force: true });
      await fs.rename(partialPath, archivePath);
      this.downloadedArchivePath = archivePath;
      this.cached = createSnapshot({
        status: "downloaded",
        currentVersion: this.cached.currentVersion,
        latestVersion: candidate.version,
        summary: `新版本 ${candidate.version} 已下载。`,
        detail: "点击重启安装后，启动器会关闭并替换本地程序文件。",
        releasePageUrl: candidate.releasePageUrl,
        updateAvailable: true,
        downloadProgress: 1,
        downloadedBytes: stat.size,
        totalBytes: candidate.size,
        artifactFileName: candidate.fileName,
        canCheck: true,
        canDownload: false,
        canInstall: true,
      });
      return this.cached;
    } catch (error) {
      await fs.rm(partialPath, { force: true });
      const detail = error instanceof Error ? error.message : "下载更新失败。";
      this.cached = createSnapshot({
        ...this.cached,
        status: "error",
        summary: "下载更新失败。",
        detail,
        updateAvailable: true,
        canCheck: true,
        canDownload: true,
        canInstall: false,
      });
      return this.cached;
    }
  }

  async installDownloadedUpdate(appProcessId: number) {
    if (this.platform !== "win32") {
      this.cached = createSnapshot({
        ...this.cached,
        status: "error",
        summary: "当前平台暂不支持自动安装。",
        detail: "首版自动安装只支持 Windows。",
        canInstall: false,
      });
      return this.cached;
    }
    if (!this.downloadedArchivePath || !this.updateCandidate) {
      this.cached = createSnapshot({
        ...this.cached,
        status: "error",
        summary: "没有可安装的更新包。",
        detail: "请先下载更新。",
        canInstall: false,
      });
      return this.cached;
    }

    const scriptPath = path.join(this.updateRoot(), "install-update.ps1");
    await fs.writeFile(scriptPath, this.buildInstallScript(appProcessId), "utf8");
    const child = spawn("powershell.exe", ["-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath], {
      detached: true,
      stdio: "ignore",
      windowsHide: true,
    });
    child.unref();

    this.cached = createSnapshot({
      ...this.cached,
      status: "installing",
      summary: "正在重启并安装更新。",
      detail: "启动器关闭后会替换程序文件，然后重新打开。",
      canCheck: false,
      canDownload: false,
      canInstall: false,
    });
    return this.cached;
  }

  private async readBuildInfo(): Promise<BuildInfo | ReleaseCheckSnapshot> {
    if (this.platform !== "win32") {
      return updateDisabled("当前平台暂不支持自动更新。");
    }

    const buildInfoPath = path.join(this.basePath, "build_info.json");
    let payload: Record<string, unknown>;
    try {
      payload = JSON.parse(await fs.readFile(buildInfoPath, "utf8")) as Record<string, unknown>;
    } catch {
      return updateDisabled("开发版本不支持更新。");
    }

    const version = stringValue(payload.version);
    const artifactId = stringValue(payload.artifact_id);
    const releaseNotesRef = stringValue(payload.release_notes_ref);
    if (!normalizeSemver(version) || !releaseNotesRef) {
      return updateDisabled("开发版本不支持更新。");
    }
    if (artifactId && artifactId !== this.targetArtifactId) {
      return updateDisabled("当前包不属于 Windows 整包，暂不支持自动更新。");
    }
    if (!resolveRepositoryUrl(releaseNotesRef)) {
      return updateDisabled("当前包元数据未声明 GitHub 发布页。");
    }

    return { artifactId, releaseNotesRef, version };
  }

  private async loadSnapshot() {
    const buildInfo = await this.readBuildInfo();
    if ("status" in buildInfo) {
      this.updateCandidate = null;
      this.downloadedArchivePath = "";
      return buildInfo;
    }

    const currentVersion = buildInfo.version;
    const current = normalizeSemver(currentVersion);
    const repositoryUrl = resolveRepositoryUrl(buildInfo.releaseNotesRef);
    try {
      const latestReleaseResponse = await this.fetchLike(
        repositoryUrl.replace("https://github.com/", "https://api.github.com/repos/") + "/releases/latest",
        {
          headers: { Accept: "application/vnd.github+json", "User-Agent": `RayleaLauncher/${currentVersion}` },
          signal: AbortSignal.timeout(this.requestTimeoutMs),
        },
      );

      if (!latestReleaseResponse.ok) {
        throw new Error(`${latestReleaseResponse.status} ${latestReleaseResponse.statusText}`);
      }

      const latestPayload = (await latestReleaseResponse.json()) as Record<string, unknown>;
      const releasePageUrl = stringValue(latestPayload.html_url) || buildInfo.releaseNotesRef;
      const assets = Array.isArray(latestPayload.assets) ? latestPayload.assets as Array<Record<string, unknown>> : [];
      const manifestAsset = assets.find((asset) =>
        stringValue(asset.name) === "release_manifest.json"
        && Boolean(stringValue(asset.browser_download_url))
      );
      if (!manifestAsset) {
        throw new Error("GitHub Release 中没有 release_manifest.json。");
      }

      const manifestResponse = await this.fetchLike(stringValue(manifestAsset.browser_download_url), {
        headers: { Accept: "application/json", "User-Agent": `RayleaLauncher/${currentVersion}` },
        signal: AbortSignal.timeout(this.requestTimeoutMs),
      });
      if (!manifestResponse.ok) {
        throw new Error(`${manifestResponse.status} ${manifestResponse.statusText}`);
      }
      const manifest = (await manifestResponse.json()) as Record<string, unknown>;
      const latestVersion = stringValue(manifest.version);
      const latest = normalizeSemver(latestVersion);
      if (!current || !latest) {
        return createSnapshot({
          status: "error",
          currentVersion,
          latestVersion,
          summary: "发布源返回的版本号无法比较。",
          detail: "请检查 release_manifest.json 中的 version 字段。",
          releasePageUrl,
          updateAvailable: false,
          canCheck: true,
        });
      }

      const artifact = this.findTargetArtifact(manifest);
      const artifactAsset = assets.find((asset) =>
        stringValue(asset.name) === artifact.fileName
        && Boolean(stringValue(asset.browser_download_url))
      );
      if (!artifactAsset) {
        throw new Error(`GitHub Release 中没有 ${artifact.fileName}。`);
      }

      const releaseNotesRef = stringValue(manifest.release_notes_ref) || releasePageUrl;
      if (compareSemver(latest, current) > 0) {
        this.updateCandidate = {
          artifactId: this.targetArtifactId,
          downloadUrl: stringValue(artifactAsset.browser_download_url),
          fileName: artifact.fileName,
          releasePageUrl: releaseNotesRef,
          sha256: artifact.sha256,
          size: artifact.size,
          version: latest,
        };
        this.downloadedArchivePath = "";
        return createSnapshot({
          status: "update_available",
          currentVersion,
          latestVersion: latest,
          summary: `发现新版本 ${latest}。`,
          detail: "可以下载 Windows 整包，下载完成后重启安装。",
          releasePageUrl: releaseNotesRef,
          updateAvailable: true,
          artifactFileName: artifact.fileName,
          totalBytes: artifact.size,
          canCheck: true,
          canDownload: true,
          canInstall: false,
        });
      }

      this.updateCandidate = null;
      this.downloadedArchivePath = "";
      return createSnapshot({
        status: "up_to_date",
        currentVersion,
        latestVersion: current,
        summary: `当前版本 ${current} 已是最新。`,
        detail: "",
        releasePageUrl: releaseNotesRef,
        updateAvailable: false,
        canCheck: true,
      });
    } catch (error) {
      const detail = error instanceof Error ? error.message : "版本源不可用。";
      return createSnapshot({
        status: "error",
        currentVersion,
        latestVersion: "",
        summary: "暂时无法连接版本源。",
        detail,
        releasePageUrl: buildInfo.releaseNotesRef,
        updateAvailable: false,
        canCheck: true,
      });
    }
  }

  private findTargetArtifact(manifest: Record<string, unknown>) {
    const artifacts = Array.isArray(manifest.artifacts) ? manifest.artifacts as Array<Record<string, unknown>> : [];
    const artifact = artifacts.find((item) => stringValue(item.artifact_id) === this.targetArtifactId);
    if (!artifact) {
      throw new Error(`release_manifest.json 中没有 ${this.targetArtifactId}。`);
    }

    const fileName = stringValue(artifact.file_name);
    const sha256 = stringValue(artifact.sha256);
    const size = numberValue(artifact.size);
    if (!fileName || !/^[0-9a-f]{64}$/i.test(sha256) || size <= 0) {
      throw new Error(`${this.targetArtifactId} 的发布元数据不完整。`);
    }
    return { fileName, sha256, size };
  }

  private downloadingSnapshot(candidate: ReleaseArtifactCandidate, downloadedBytes: number, totalBytes: number) {
    return createSnapshot({
      ...this.cached,
      status: "downloading",
      latestVersion: candidate.version,
      summary: `正在下载 ${candidate.version}。`,
      detail: candidate.fileName,
      releasePageUrl: candidate.releasePageUrl,
      updateAvailable: true,
      downloadProgress: totalBytes > 0 ? Math.min(1, downloadedBytes / totalBytes) : null,
      downloadedBytes,
      totalBytes,
      artifactFileName: candidate.fileName,
      canCheck: false,
      canDownload: false,
      canInstall: false,
    });
  }

  private updateRoot() {
    return path.join(this.basePath, "cache", "downloads", "updates");
  }

  private buildInstallScript(appProcessId: number) {
    const installRoot = path.resolve(this.basePath);
    const updateRoot = path.resolve(this.updateRoot());
    const archivePath = path.resolve(this.downloadedArchivePath);
    const launcherPath = path.join(installRoot, "RayleaLauncher.exe");
    const preservedArray = PRESERVED_TOP_LEVEL_DIRS.map(psSingleQuoted).join(", ");

    return `
$ErrorActionPreference = 'Stop'
$installRoot = [System.IO.Path]::GetFullPath(${psSingleQuoted(installRoot)})
$updateRoot = [System.IO.Path]::GetFullPath(${psSingleQuoted(updateRoot)})
$archivePath = [System.IO.Path]::GetFullPath(${psSingleQuoted(archivePath)})
$launcherPath = [System.IO.Path]::GetFullPath(${psSingleQuoted(launcherPath)})
$preservedTopLevelDirs = @(${preservedArray})
function Test-IsInside($candidate, $root) {
  $candidateFull = [System.IO.Path]::GetFullPath($candidate)
  $rootPrefix = [System.IO.Path]::GetFullPath($root)
  if (-not $rootPrefix.EndsWith([System.IO.Path]::DirectorySeparatorChar)) { $rootPrefix += [System.IO.Path]::DirectorySeparatorChar }
  return $candidateFull.StartsWith($rootPrefix, [System.StringComparison]::OrdinalIgnoreCase)
}
if (-not (Test-Path -LiteralPath $installRoot -PathType Container)) { throw "install root not found" }
if (-not (Test-Path -LiteralPath $archivePath -PathType Leaf)) { throw "update archive not found" }
if (-not (Test-IsInside $archivePath $updateRoot)) { throw "update archive is outside update cache" }
$extractRoot = Join-Path $updateRoot 'install-extract'
if (Test-Path -LiteralPath $extractRoot) { Remove-Item -LiteralPath $extractRoot -Recurse -Force }
New-Item -ItemType Directory -Path $extractRoot -Force | Out-Null
Expand-Archive -LiteralPath $archivePath -DestinationPath $extractRoot -Force
$payloadRoots = @(Get-ChildItem -LiteralPath $extractRoot -Directory)
if ($payloadRoots.Count -ne 1) { throw "update archive must contain exactly one root directory" }
$payloadRoot = [System.IO.Path]::GetFullPath($payloadRoots[0].FullName)
try { Wait-Process -Id ${appProcessId} -Timeout 120 } catch {}
foreach ($child in Get-ChildItem -LiteralPath $payloadRoot -Force) {
  if ($preservedTopLevelDirs -contains $child.Name) { continue }
  $target = [System.IO.Path]::GetFullPath((Join-Path $installRoot $child.Name))
  if (-not (Test-IsInside $target $installRoot)) { throw "target path escaped install root" }
  if ($child.Name -eq 'config') {
    New-Item -ItemType Directory -Path $target -Force | Out-Null
    foreach ($configChild in Get-ChildItem -LiteralPath $child.FullName -Force) {
      if ($configChild.Name -eq 'user.yaml') { continue }
      $configTarget = [System.IO.Path]::GetFullPath((Join-Path $target $configChild.Name))
      if (-not (Test-IsInside $configTarget $target)) { throw "config target escaped config root" }
      if (Test-Path -LiteralPath $configTarget) { Remove-Item -LiteralPath $configTarget -Recurse -Force }
      Copy-Item -LiteralPath $configChild.FullName -Destination $configTarget -Recurse -Force
    }
    continue
  }
  if (Test-Path -LiteralPath $target) { Remove-Item -LiteralPath $target -Recurse -Force }
  Copy-Item -LiteralPath $child.FullName -Destination $target -Recurse -Force
}
if (Test-Path -LiteralPath $launcherPath -PathType Leaf) {
  Start-Process -FilePath $launcherPath -WorkingDirectory $installRoot -WindowStyle Hidden
}
`.trimStart();
  }
}

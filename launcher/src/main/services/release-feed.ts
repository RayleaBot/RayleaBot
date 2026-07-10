import { execFile, spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { constants as fsConstants } from "node:fs";
import fs from "node:fs/promises";
import path from "node:path";
import type { LauncherResolvedSettings, ReleaseCheckSnapshot } from "../../shared/launcher-models";
import { createReleaseDisabled, createReleaseUnavailable } from "../../shared/launcher-copy";

const DEFAULT_CACHE_TTL_MS = 6 * 60 * 60 * 1000;
const HELPER_TIMEOUT_MS = 30 * 60 * 1000;
const MAX_HELPER_OUTPUT_BYTES = 4 * 1024 * 1024;
const MANIFEST_FILE_NAME = "release_manifest.v2.json";
const SIGNATURE_FILE_NAME = "release_manifest.v2.sig.json";
const SEMVER_PATTERN = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/;

interface TrustedArtifact {
  artifact_id: string;
  file_name: string;
  archive_size_bytes: number;
  update_mode: "automatic" | "guided" | "manual";
}

interface TrustedCheckResult {
  status: "up_to_date" | "update_available";
  current_version: string;
  available_version?: string;
  update_mode: "automatic" | "guided" | "manual";
  release_page_url?: string;
  automatic_install_supported: boolean;
  manifest_path?: string;
  signature_path?: string;
  artifact: TrustedArtifact;
}

interface TrustedDownloadResult extends TrustedCheckResult {
  artifact_path: string;
}

interface PackagedBuildInfo {
  version: string;
  artifact_id: string;
  update_protocol_version: number;
}

export interface UpdaterCommandResult {
  stdout: string;
  stderr: string;
}

export interface LauncherUpdaterRunner {
  run(executable: string, args: string[]): Promise<UpdaterCommandResult>;
  spawnDetached(executable: string, args: string[]): Promise<void>;
}

export interface LauncherReleaseFeedClientOptions {
  cacheTtlMs?: number;
  platform?: NodeJS.Platform;
  runner?: LauncherUpdaterRunner;
  updaterPath?: string;
}

function defaultUpdaterRunner(): LauncherUpdaterRunner {
  return {
    run(executable, args) {
      return new Promise((resolve, reject) => {
        execFile(
          executable,
          args,
          {
            encoding: "utf8",
            maxBuffer: MAX_HELPER_OUTPUT_BYTES,
            timeout: HELPER_TIMEOUT_MS,
            windowsHide: true,
          },
          (error, stdout, stderr) => {
            if (error) {
              reject(new Error(safeHelperFailure(stderr, error.message)));
              return;
            }
            resolve({ stdout, stderr });
          },
        );
      });
    },
    spawnDetached(executable, args) {
      return new Promise((resolve, reject) => {
        const child = spawn(executable, args, {
          detached: true,
          stdio: "ignore",
          windowsHide: true,
        });
        child.once("spawn", () => {
          child.unref();
          resolve();
        });
        child.once("error", reject);
      });
    },
  };
}

function safeHelperFailure(stderr: string, fallback: string) {
  try {
    const payload = JSON.parse(stderr.trim()) as Record<string, unknown>;
    const code = stringValue(payload.code);
    if (code) {
      return `更新助手拒绝了操作（${code}）。`;
    }
  } catch {
    // The helper's raw stderr can contain local paths. Do not surface it to the renderer.
  }
  return fallback ? "更新助手执行失败。" : "更新助手未返回结果。";
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function numberValue(value: unknown) {
  return typeof value === "number" && Number.isSafeInteger(value) ? value : 0;
}

function parseArtifact(value: unknown): TrustedArtifact {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error("更新助手返回了无效 artifact。");
  }
  const input = value as Record<string, unknown>;
  const artifactId = stringValue(input.artifact_id);
  const fileName = stringValue(input.file_name);
  const archiveSize = numberValue(input.archive_size_bytes);
  const updateMode = stringValue(input.update_mode);
  if (
    artifactId !== "windows-x64-full"
    || !/^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$/.test(fileName)
    || archiveSize <= 0
    || archiveSize > 2 * 1024 * 1024 * 1024
    || !["automatic", "guided", "manual"].includes(updateMode)
  ) {
    throw new Error("更新助手返回了无效 artifact。");
  }
  return {
    artifact_id: artifactId,
    file_name: fileName,
    archive_size_bytes: archiveSize,
    update_mode: updateMode as TrustedArtifact["update_mode"],
  };
}

function parseCheckResult(stdout: string): TrustedCheckResult {
  const payload = JSON.parse(stdout) as Record<string, unknown>;
  const status = stringValue(payload.status);
  const updateMode = stringValue(payload.update_mode);
  const currentVersion = stringValue(payload.current_version);
  const availableVersion = stringValue(payload.available_version);
  const automaticInstallSupported = payload.automatic_install_supported === true;
  const releasePageUrl = stringValue(payload.release_page_url);
  const artifact = parseArtifact(payload.artifact);
  if (
    !["up_to_date", "update_available"].includes(status)
    || !["automatic", "guided", "manual"].includes(updateMode)
    || !SEMVER_PATTERN.test(currentVersion)
    || (status === "update_available" && !SEMVER_PATTERN.test(availableVersion))
    || (releasePageUrl && !isSafeReleaseURL(releasePageUrl))
    || artifact.update_mode !== updateMode
    || (automaticInstallSupported && updateMode !== "automatic")
  ) {
    throw new Error("更新助手返回了无效检查结果。");
  }
  return {
    status: status as TrustedCheckResult["status"],
    current_version: currentVersion,
    available_version: availableVersion || undefined,
    update_mode: updateMode as TrustedCheckResult["update_mode"],
    release_page_url: releasePageUrl || undefined,
    automatic_install_supported: automaticInstallSupported,
    manifest_path: stringValue(payload.manifest_path) || undefined,
    signature_path: stringValue(payload.signature_path) || undefined,
    artifact,
  };
}

function parseDownloadResult(stdout: string): TrustedDownloadResult {
  const checked = parseCheckResult(stdout);
  const payload = JSON.parse(stdout) as Record<string, unknown>;
  const artifactPath = stringValue(payload.artifact_path);
  if (!artifactPath) {
    throw new Error("更新助手未返回下载包路径。");
  }
  return { ...checked, artifact_path: artifactPath };
}

function createSnapshot(input: Partial<ReleaseCheckSnapshot>): ReleaseCheckSnapshot {
  const status = input.status ?? "disabled";
  const busy = status === "checking" || status === "downloading" || status === "installing";
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
    canCheck: input.canCheck ?? (!busy && status !== "disabled"),
    canDownload: input.canDownload ?? status === "update_available",
    canInstall: input.canInstall ?? status === "ready_to_install",
  };
}

function isSafeReleaseURL(value: string) {
  try {
    const parsed = new URL(value);
    return parsed.protocol === "https:" && parsed.username === "" && parsed.password === "";
  } catch {
    return false;
  }
}

async function readPackagedBuildInfo(basePath: string): Promise<PackagedBuildInfo | null> {
  try {
    const payload = JSON.parse(await fs.readFile(path.join(basePath, "build_info.json"), "utf8")) as Record<string, unknown>;
    const version = stringValue(payload.version);
    const artifactId = stringValue(payload.artifact_id);
    const updateProtocolVersion = numberValue(payload.update_protocol_version);
    if (!version || artifactId !== "windows-x64-full" || updateProtocolVersion < 2) {
      return null;
    }
    return { version, artifact_id: artifactId, update_protocol_version: updateProtocolVersion };
  } catch {
    return null;
  }
}

async function assertRegularFile(filePath: string) {
  const info = await fs.lstat(filePath);
  if (!info.isFile() || info.isSymbolicLink()) {
    throw new Error("更新事务输入不是普通文件。");
  }
}

export class LauncherReleaseFeedClient {
  private cachedAt = 0;
  private cached: ReleaseCheckSnapshot = createSnapshot(createReleaseUnavailable("尚未检查版本。"));
  private readonly cacheTtlMs: number;
  private readonly platform: NodeJS.Platform;
  private readonly runner: LauncherUpdaterRunner;
  private readonly updaterPath: string;
  private checked: TrustedCheckResult | null = null;
  private downloaded: TrustedDownloadResult | null = null;

  constructor(private readonly basePath: string, options: LauncherReleaseFeedClientOptions = {}) {
    this.cacheTtlMs = options.cacheTtlMs ?? DEFAULT_CACHE_TTL_MS;
    this.platform = options.platform ?? process.platform;
    this.runner = options.runner ?? defaultUpdaterRunner();
    this.updaterPath = options.updaterPath ?? path.join(basePath, "raylea-updater.exe");
  }

  async getSnapshot(options: { force?: boolean } = {}) {
    if (!options.force && Date.now() - this.cachedAt < this.cacheTtlMs) {
      return this.cached;
    }
    const buildInfo = await readPackagedBuildInfo(this.basePath);
    if (!buildInfo) {
      this.cached = createSnapshot(createReleaseDisabled("当前安装缺少可信更新基线，需要手动安装首个 v2 正式包。"));
      this.cachedAt = Date.now();
      return this.cached;
    }
    if (this.platform !== "win32") {
      this.cached = createSnapshot(createReleaseDisabled("当前平台使用签名校验后的引导更新；自动安装首批仅支持 Windows x64 整包。"));
      this.cached.currentVersion = buildInfo.version;
      this.cachedAt = Date.now();
      return this.cached;
    }
    try {
      await assertRegularFile(this.updaterPath);
      const { stdout } = await this.runner.run(this.updaterPath, ["check", "--install-root", this.basePath, "--json"]);
      const result = parseCheckResult(stdout);
      this.checked = result;
      this.downloaded = null;
      this.cached = this.snapshotFromCheck(result);
    } catch (error) {
      this.checked = null;
      this.downloaded = null;
      this.cached = createSnapshot({
        status: "failed",
        currentVersion: buildInfo.version,
        summary: "无法确认受信任的更新。",
        detail: error instanceof Error ? error.message : "更新助手执行失败。",
        canCheck: true,
        canDownload: false,
        canInstall: false,
      });
    }
    this.cachedAt = Date.now();
    return this.cached;
  }

  async downloadUpdate(onProgress?: (snapshot: ReleaseCheckSnapshot) => void | Promise<void>) {
    if (!this.checked || this.checked.status !== "update_available") {
      await this.getSnapshot({ force: true });
    }
    if (!this.checked || !this.checked.automatic_install_supported || this.checked.update_mode !== "automatic") {
      return this.cached;
    }
    const previous = this.cached;
    await onProgress?.(createSnapshot({
      ...previous,
      status: "downloading",
      summary: `正在下载 ${previous.latestVersion}。`,
      detail: "下载完成后仍会再次校验签名、摘要和 Authenticode。",
      canCheck: false,
      canDownload: false,
      canInstall: false,
    }));
    try {
      const { stdout } = await this.runner.run(this.updaterPath, ["download", "--install-root", this.basePath, "--json"]);
      const result = parseDownloadResult(stdout);
      if (!result.automatic_install_supported || result.update_mode !== "automatic") {
        throw new Error("受信任清单未授权自动安装。");
      }
      this.checked = result;
      this.downloaded = result;
      this.cached = createSnapshot({
        status: "ready_to_install",
        currentVersion: result.current_version,
        latestVersion: result.available_version ?? "",
        summary: `新版本 ${result.available_version ?? ""} 已验证并准备安装。`,
        detail: "确认安装后会停服、离线备份、事务替换并在失败时自动回滚。",
        releasePageUrl: result.release_page_url ?? "",
        updateAvailable: true,
        downloadProgress: 1,
        downloadedBytes: result.artifact.archive_size_bytes,
        totalBytes: result.artifact.archive_size_bytes,
        artifactFileName: result.artifact.file_name,
        canCheck: true,
        canDownload: false,
        canInstall: true,
      });
      return this.cached;
    } catch (error) {
      this.downloaded = null;
      this.cached = createSnapshot({
        ...previous,
        status: "failed",
        summary: "下载或校验更新失败。",
        detail: error instanceof Error ? error.message : "更新助手执行失败。",
        canCheck: true,
        canDownload: true,
        canInstall: false,
      });
      return this.cached;
    }
  }

  async installDownloadedUpdate(
    appProcessId: number,
    serviceWasRunning: boolean,
    settings: LauncherResolvedSettings,
  ) {
    if (this.platform !== "win32" || !this.downloaded || !this.downloaded.automatic_install_supported) {
      this.cached = createSnapshot({
        ...this.cached,
        status: "failed",
        summary: "没有可自动安装的受信任更新。",
        detail: "请重新检查并下载更新，或按发布页指引手动升级。",
        canInstall: false,
      });
      return this.cached;
    }
    const officialServer = path.join(this.basePath, "raylea-server.exe");
    const officialConfig = path.join(this.basePath, "config", "user.yaml");
    if (
      !sameLocalPath(settings.installationRoot, this.basePath)
      || !sameLocalPath(settings.serverExecutablePath, officialServer)
      || !sameLocalPath(settings.configPath, officialConfig)
    ) {
      this.cached = createSnapshot({
        ...this.cached,
        status: "failed",
        summary: "当前运行方式不支持自动安装。",
        detail: "检测到自定义安装根、服务程序或配置路径，请按发布页指引手动升级。",
        canInstall: false,
      });
      return this.cached;
    }

    const transactionRoot = path.join(
      path.dirname(path.resolve(this.basePath)),
      `.rayleabot-update-${Date.now()}-${randomBytes(8).toString("hex")}`,
    );
    try {
      const manifestSource = this.downloaded.manifest_path;
      const signatureSource = this.downloaded.signature_path;
      if (!manifestSource || !signatureSource) {
        throw new Error("更新助手未保留已验证元数据。");
      }
      const sources = [this.updaterPath, manifestSource, signatureSource, this.downloaded.artifact_path];
      await Promise.all(sources.map(assertRegularFile));
      const cacheRoot = path.join(this.basePath, "cache", "downloads", "updates");
      for (const source of [manifestSource, signatureSource, this.downloaded.artifact_path]) {
        if (!pathInside(cacheRoot, source)) {
          throw new Error("更新事务输入位于受控下载缓存之外。");
        }
      }
      await fs.mkdir(transactionRoot, { recursive: false, mode: 0o700 });
      const externalHelper = path.join(transactionRoot, "raylea-updater.exe");
      const manifestPath = path.join(transactionRoot, MANIFEST_FILE_NAME);
      const signaturePath = path.join(transactionRoot, SIGNATURE_FILE_NAME);
      const artifactPath = path.join(transactionRoot, this.downloaded.artifact.file_name);
      await fs.copyFile(this.updaterPath, externalHelper, fsConstants.COPYFILE_EXCL);
      await fs.copyFile(manifestSource, manifestPath, fsConstants.COPYFILE_EXCL);
      await fs.copyFile(signatureSource, signaturePath, fsConstants.COPYFILE_EXCL);
      await fs.copyFile(this.downloaded.artifact_path, artifactPath, fsConstants.COPYFILE_EXCL);

      const args = [
        "install",
        "--install-root", this.basePath,
        "--transaction-root", transactionRoot,
        "--manifest", manifestPath,
        "--signature", signaturePath,
        "--artifact", artifactPath,
        "--launcher-pid", String(appProcessId),
        `--service-was-running=${serviceWasRunning ? "true" : "false"}`,
      ];
      await this.runner.spawnDetached(externalHelper, args);
      this.cached = createSnapshot({
        ...this.cached,
        status: "installing",
        summary: "正在准备事务式安装。",
        detail: "Launcher 退出后，外置更新助手会完成备份、替换、健康检查和必要的回滚。",
        canCheck: false,
        canDownload: false,
        canInstall: false,
      });
      return this.cached;
    } catch (error) {
      await fs.rm(transactionRoot, { recursive: true, force: true });
      this.cached = createSnapshot({
        ...this.cached,
        status: "failed",
        summary: "无法启动事务式安装。",
        detail: error instanceof Error ? error.message : "更新助手启动失败。",
        canCheck: true,
        canInstall: this.downloaded !== null,
      });
      return this.cached;
    }
  }

  private snapshotFromCheck(result: TrustedCheckResult) {
    const updateAvailable = result.status === "update_available";
    const automatic = updateAvailable && result.update_mode === "automatic" && result.automatic_install_supported;
    const summary = updateAvailable
      ? `发现新版本 ${result.available_version ?? ""}。`
      : `当前版本 ${result.current_version} 已是最新。`;
    const detail = updateAvailable
      ? automatic
        ? "用户确认后才会下载和安装；安装前后均执行完整信任校验。"
        : "此发布仅提供引导更新，请打开发布页按说明手动升级。"
      : "";
    return createSnapshot({
      status: result.status,
      currentVersion: result.current_version,
      latestVersion: result.available_version ?? result.current_version,
      summary,
      detail,
      releasePageUrl: result.release_page_url ?? "",
      updateAvailable,
      totalBytes: result.artifact.archive_size_bytes,
      artifactFileName: result.artifact.file_name,
      canCheck: true,
      canDownload: automatic,
      canInstall: false,
    });
  }
}

function sameLocalPath(left: string, right: string) {
  return path.resolve(left).toLowerCase() === path.resolve(right).toLowerCase();
}

function pathInside(root: string, candidate: string) {
  const relative = path.relative(path.resolve(root), path.resolve(candidate));
  return relative !== "" && relative !== ".." && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
}

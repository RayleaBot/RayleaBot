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

export interface ReleaseFailureDescription {
  errorCode: string;
  summary: string;
  detail: string;
}

interface ReleaseFailureContext {
  installRoot?: string;
  updaterPath?: string;
  transactionRoot?: string;
}

const RELEASE_ERROR_SUMMARIES: Readonly<Record<string, string>> = {
  "release.trust_required": "当前安装不具备自动更新信任基线",
  "release.manifest_invalid": "发布清单无效",
  "release.signature_invalid": "发布签名验证失败",
  "release.manifest_expired": "发布清单已过期",
  "release.replay_rejected": "已拒绝旧版或被替换的发布清单",
  "release.artifact_invalid": "更新包完整性或签名验证失败",
  "release.update_not_supported": "当前平台仅支持引导更新",
  "release.disk_space_insufficient": "磁盘空间不足",
  "release.install_failed": "更新安装失败",
  "release.rollback_failed": "更新回滚失败",
};

class ReleaseUpdateError extends Error {
  constructor(
    readonly errorCode: string,
    readonly summary: string,
    detail: string,
  ) {
    super(detail);
    this.name = "ReleaseUpdateError";
  }
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
              reject(updaterCommandFailure(stderr, error));
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

function systemErrorCode(error: unknown) {
  if (!error || typeof error !== "object" || !("code" in error)) {
    return "";
  }
  const code = (error as { code?: unknown }).code;
  if (typeof code === "string" && code.trim()) {
    return code.trim();
  }
  if (typeof code === "number" && Number.isFinite(code)) {
    return `exit.${code}`;
  }
  return "";
}

function firstDiagnosticLine(value: string) {
  return value.split(/\r?\n/, 1)[0]?.trim() ?? "";
}

function releaseSummary(errorCode: string, detail: string) {
  return RELEASE_ERROR_SUMMARIES[errorCode]
    ?? (firstDiagnosticLine(detail) || `更新助手报告错误 ${errorCode}`);
}

function unstructuredUpdaterFailure(fallback: Error) {
  const processError = fallback as Error & {
    code?: unknown;
    killed?: unknown;
    signal?: unknown;
  };
  const errorCode = systemErrorCode(fallback);
  if (processError.killed === true) {
    return new ReleaseUpdateError(
      "launcher.update_helper_timeout",
      "更新助手执行超时",
      `更新助手在 ${HELPER_TIMEOUT_MS / 60_000} 分钟内没有完成，进程已被终止。`,
    );
  }
  if (errorCode === "ENOENT") {
    return new ReleaseUpdateError(errorCode, "更新助手不存在", "系统无法找到更新助手可执行文件。");
  }
  if (errorCode === "EACCES" || errorCode === "EPERM") {
    return new ReleaseUpdateError(errorCode, "更新助手无法执行", `系统拒绝执行更新助手（${errorCode}）。`);
  }
  if (errorCode === "ERR_CHILD_PROCESS_STDIO_MAXBUFFER") {
    return new ReleaseUpdateError(errorCode, "更新助手输出超过限制", "更新助手输出超过 4 MB，执行已终止。");
  }
  if (errorCode.startsWith("exit.")) {
    const exitCode = errorCode.slice("exit.".length);
    return new ReleaseUpdateError(
      errorCode,
      `更新助手以退出代码 ${exitCode} 结束`,
      `更新助手以退出代码 ${exitCode} 结束，但没有返回可解析的结构化错误。`,
    );
  }
  const signal = stringValue(processError.signal);
  if (signal) {
    return new ReleaseUpdateError(
      "launcher.update_helper_signal",
      "更新助手被系统终止",
      `更新助手被信号 ${signal} 终止，且没有返回可解析的结构化错误。`,
    );
  }
  if (errorCode) {
    return new ReleaseUpdateError(
      errorCode,
      `更新助手执行失败（${errorCode}）`,
      `系统返回错误代码 ${errorCode}，且更新助手没有返回可解析的结构化错误。`,
    );
  }
  return new ReleaseUpdateError(
    "launcher.update_helper_failed",
    "更新助手执行失败",
    "更新助手没有返回错误代码或可解析的结构化错误。",
  );
}

export function updaterCommandFailure(stderr: string, fallback: Error) {
  try {
    const payload = JSON.parse(stderr.trim()) as Record<string, unknown>;
    const code = stringValue(payload.code);
    const detail = stringValue(payload.error);
    if (code || detail) {
      const errorCode = code || systemErrorCode(fallback) || "launcher.update_helper_failed";
      return new ReleaseUpdateError(
        errorCode,
        releaseSummary(errorCode, detail),
        detail || fallback.message || "更新助手没有返回错误原因。",
      );
    }
  } catch {
    // Non-JSON stderr is not exposed because it can contain arbitrary process output.
  }
  return unstructuredUpdaterFailure(fallback);
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function redactKnownPath(value: string, candidate: string, replacement: string) {
  if (!candidate.trim()) {
    return value;
  }
  const resolved = path.resolve(candidate);
  const pattern = resolved
    .split(/[\\/]+/)
    .filter(Boolean)
    .map(escapeRegExp)
    .join("[\\\\/]+");
  return pattern ? value.replace(new RegExp(pattern, "gi"), replacement) : value;
}

function sanitizeReleaseDetail(value: string, context: ReleaseFailureContext) {
  let detail = value
    .replace(/[\u0000-\u0008\u000B\u000C\u000E-\u001F\u007F]/g, "")
    .trim();
  if (context.updaterPath) {
    detail = redactKnownPath(detail, context.updaterPath, "raylea-updater.exe");
  }
  if (context.installRoot) {
    detail = redactKnownPath(detail, context.installRoot, "<安装目录>");
  }
  if (context.transactionRoot) {
    detail = redactKnownPath(detail, context.transactionRoot, "<更新事务目录>");
  }
  const maximumLength = 2000;
  return detail.length > maximumLength
    ? `${detail.slice(0, maximumLength).trimEnd()}…`
    : detail;
}

export function describeReleaseFailure(
  error: unknown,
  context: ReleaseFailureContext = {},
): ReleaseFailureDescription {
  const rawDetail = error instanceof Error ? error.message : String(error ?? "");
  const detail = sanitizeReleaseDetail(rawDetail, context) || "更新检查没有返回错误原因。";
  if (error instanceof ReleaseUpdateError) {
    return {
      errorCode: error.errorCode,
      summary: sanitizeReleaseDetail(error.summary, context) || releaseSummary(error.errorCode, detail),
      detail,
    };
  }

  const errorCode = systemErrorCode(error);
  switch (errorCode) {
    case "ENOENT":
      return { errorCode, summary: "更新所需文件不存在", detail };
    case "EACCES":
    case "EPERM":
      return { errorCode, summary: "更新所需文件无法访问", detail };
    case "ETIMEDOUT":
      return { errorCode, summary: "更新检查超时", detail };
    case "ERR_CHILD_PROCESS_STDIO_MAXBUFFER":
      return { errorCode, summary: "更新助手输出超过限制", detail };
    default:
      return {
        errorCode: errorCode || (error instanceof SyntaxError ? "launcher.update_response_invalid" : "launcher.update_unknown"),
        summary: firstDiagnosticLine(detail) || "更新检查没有返回错误摘要",
        detail,
      };
  }
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function numberValue(value: unknown) {
  return typeof value === "number" && Number.isSafeInteger(value) ? value : 0;
}

function invalidUpdaterResponse(errorCode: string, summary: string, detail: string) {
  return new ReleaseUpdateError(errorCode, summary, detail);
}

function parseUpdaterJSON(stdout: string) {
  try {
    const payload = JSON.parse(stdout) as unknown;
    if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
      throw new Error("顶层结果不是对象");
    }
    return payload as Record<string, unknown>;
  } catch (error) {
    const reason = error instanceof Error ? error.message : "无法解析 JSON";
    throw invalidUpdaterResponse(
      "launcher.update_response_invalid",
      "更新助手返回的结果无法解析",
      `更新助手没有返回有效 JSON：${reason}。`,
    );
  }
}

function parseArtifact(value: unknown): TrustedArtifact {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw invalidUpdaterResponse(
      "launcher.update_artifact_response_invalid",
      "更新助手返回的更新包信息无效",
      "artifact 字段必须是对象。",
    );
  }
  const input = value as Record<string, unknown>;
  const artifactId = stringValue(input.artifact_id);
  const fileName = stringValue(input.file_name);
  const archiveSize = numberValue(input.archive_size_bytes);
  const updateMode = stringValue(input.update_mode);
  const issues: string[] = [];
  if (artifactId !== "windows-x64-full") {
    issues.push("artifact_id 不是 windows-x64-full");
  }
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$/.test(fileName)) {
    issues.push("file_name 无效");
  }
  if (archiveSize <= 0 || archiveSize > 2 * 1024 * 1024 * 1024) {
    issues.push("archive_size_bytes 超出允许范围");
  }
  if (!["automatic", "guided", "manual"].includes(updateMode)) {
    issues.push("update_mode 无效");
  }
  if (issues.length > 0) {
    throw invalidUpdaterResponse(
      "launcher.update_artifact_response_invalid",
      "更新助手返回的更新包信息无效",
      `${issues.join("；")}。`,
    );
  }
  return {
    artifact_id: artifactId,
    file_name: fileName,
    archive_size_bytes: archiveSize,
    update_mode: updateMode as TrustedArtifact["update_mode"],
  };
}

function parseCheckResult(stdout: string): TrustedCheckResult {
  const payload = parseUpdaterJSON(stdout);
  const status = stringValue(payload.status);
  const updateMode = stringValue(payload.update_mode);
  const currentVersion = stringValue(payload.current_version);
  const availableVersion = stringValue(payload.available_version);
  const automaticInstallSupported = payload.automatic_install_supported === true;
  const releasePageUrl = stringValue(payload.release_page_url);
  const artifact = parseArtifact(payload.artifact);
  const issues: string[] = [];
  if (!["up_to_date", "update_available"].includes(status)) {
    issues.push("status 无效");
  }
  if (!["automatic", "guided", "manual"].includes(updateMode)) {
    issues.push("update_mode 无效");
  }
  if (!SEMVER_PATTERN.test(currentVersion)) {
    issues.push("current_version 不是有效语义版本");
  }
  if (status === "update_available" && !SEMVER_PATTERN.test(availableVersion)) {
    issues.push("available_version 不是有效语义版本");
  }
  if (releasePageUrl && !isSafeReleaseURL(releasePageUrl)) {
    issues.push("release_page_url 不是安全的 HTTPS 地址");
  }
  if (artifact.update_mode !== updateMode) {
    issues.push("artifact.update_mode 与顶层 update_mode 不一致");
  }
  if (automaticInstallSupported && updateMode !== "automatic") {
    issues.push("automatic_install_supported 与 update_mode 冲突");
  }
  if (issues.length > 0) {
    throw invalidUpdaterResponse(
      "launcher.update_response_invalid",
      "更新助手返回的检查结果无效",
      `${issues.join("；")}。`,
    );
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
  const payload = parseUpdaterJSON(stdout);
  const artifactPath = stringValue(payload.artifact_path);
  if (!artifactPath) {
    throw invalidUpdaterResponse(
      "launcher.update_artifact_path_missing",
      "更新助手没有返回下载包路径",
      "下载结果缺少 artifact_path。",
    );
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
    errorCode: input.errorCode ?? "",
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

async function readPackagedBuildInfo(basePath: string): Promise<PackagedBuildInfo> {
  let source: string;
  try {
    source = await fs.readFile(path.join(basePath, "build_info.json"), "utf8");
  } catch (error) {
    const failure = describeReleaseFailure(error, { installRoot: basePath });
    throw new ReleaseUpdateError(
      "release.trust_required",
      RELEASE_ERROR_SUMMARIES["release.trust_required"]!,
      systemErrorCode(error) === "ENOENT"
        ? "发布包缺少 build_info.json，请完整解压或重新下载正式包。"
        : `无法读取 build_info.json：${failure.detail}`,
    );
  }

  let payload: Record<string, unknown>;
  try {
    const parsed = JSON.parse(source) as unknown;
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      throw new Error("顶层内容不是对象");
    }
    payload = parsed as Record<string, unknown>;
  } catch (error) {
    const reason = error instanceof Error ? error.message : "无法解析 JSON";
    throw new ReleaseUpdateError(
      "release.trust_required",
      RELEASE_ERROR_SUMMARIES["release.trust_required"]!,
      `build_info.json 不是有效 JSON：${reason}。`,
    );
  }

  const version = stringValue(payload.version);
  const artifactId = stringValue(payload.artifact_id);
  const updateProtocolVersion = numberValue(payload.update_protocol_version);
  const issues: string[] = [];
  if (!SEMVER_PATTERN.test(version)) {
    issues.push("version 不是有效语义版本");
  }
  if (artifactId !== "windows-x64-full") {
    issues.push("artifact_id 不是 windows-x64-full");
  }
  if (updateProtocolVersion < 2) {
    issues.push("update_protocol_version 低于 2");
  }
  if (issues.length > 0) {
    throw new ReleaseUpdateError(
      "release.trust_required",
      RELEASE_ERROR_SUMMARIES["release.trust_required"]!,
      `build_info.json 未提供可信更新基线：${issues.join("；")}。`,
    );
  }
  return { version, artifact_id: artifactId, update_protocol_version: updateProtocolVersion };
}

async function assertRegularFile(filePath: string, label = "更新事务输入", codePrefix = "launcher.update_input") {
  let info;
  try {
    info = await fs.lstat(filePath);
  } catch (error) {
    const errorCode = systemErrorCode(error);
    if (errorCode === "ENOENT") {
      throw new ReleaseUpdateError(`${codePrefix}_missing`, `${label}不存在`, `${label}文件不存在。`);
    }
    if (errorCode === "EACCES" || errorCode === "EPERM") {
      throw new ReleaseUpdateError(`${codePrefix}_access_denied`, `${label}无法访问`, `系统拒绝访问${label}。`);
    }
    throw error;
  }
  if (!info.isFile() || info.isSymbolicLink()) {
    const reason = info.isSymbolicLink() ? "不允许使用符号链接" : "目标不是普通文件";
    throw new ReleaseUpdateError(`${codePrefix}_invalid`, `${label}无效`, `${label}${reason}。`);
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
    let buildInfo: PackagedBuildInfo;
    try {
      buildInfo = await readPackagedBuildInfo(this.basePath);
    } catch (error) {
      const failure = describeReleaseFailure(error, {
        installRoot: this.basePath,
        updaterPath: this.updaterPath,
      });
      this.checked = null;
      this.downloaded = null;
      this.cached = createSnapshot({
        status: failure.errorCode === "release.trust_required" ? "disabled" : "failed",
        summary: failure.summary,
        detail: failure.detail,
        errorCode: failure.errorCode,
        canCheck: false,
        canDownload: false,
        canInstall: false,
      });
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
      await assertRegularFile(this.updaterPath, "更新助手", "launcher.update_helper");
      const { stdout } = await this.runner.run(this.updaterPath, ["check", "--install-root", this.basePath, "--json"]);
      const result = parseCheckResult(stdout);
      this.checked = result;
      this.downloaded = null;
      this.cached = this.snapshotFromCheck(result);
    } catch (error) {
      const failure = describeReleaseFailure(error, {
        installRoot: this.basePath,
        updaterPath: this.updaterPath,
      });
      this.checked = null;
      this.downloaded = null;
      this.cached = createSnapshot({
        status: "failed",
        currentVersion: buildInfo.version,
        summary: failure.summary,
        detail: failure.detail,
        errorCode: failure.errorCode,
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
      errorCode: "",
      canCheck: false,
      canDownload: false,
      canInstall: false,
    }));
    try {
      const { stdout } = await this.runner.run(this.updaterPath, ["download", "--install-root", this.basePath, "--json"]);
      const result = parseDownloadResult(stdout);
      if (!result.automatic_install_supported || result.update_mode !== "automatic") {
        throw new ReleaseUpdateError(
          "release.update_not_supported",
          RELEASE_ERROR_SUMMARIES["release.update_not_supported"]!,
          "受信任发布清单没有授权自动安装此更新包。",
        );
      }
      this.checked = result;
      this.downloaded = result;
      this.cached = createSnapshot({
        status: "ready_to_install",
        currentVersion: result.current_version,
        latestVersion: result.available_version ?? "",
        summary: `新版本 ${result.available_version ?? ""} 已验证并准备安装。`,
        detail: "确认安装后会停服、离线备份、事务替换并在失败时自动回滚。",
        errorCode: "",
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
      const failure = describeReleaseFailure(error, {
        installRoot: this.basePath,
        updaterPath: this.updaterPath,
      });
      this.downloaded = null;
      this.cached = createSnapshot({
        ...previous,
        status: "failed",
        summary: failure.summary,
        detail: failure.detail,
        errorCode: failure.errorCode,
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
        errorCode: "release.update_not_supported",
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
        errorCode: "release.update_not_supported",
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
        throw new ReleaseUpdateError(
          "launcher.update_metadata_missing",
          "已验证的更新元数据不存在",
          "更新助手没有保留发布清单或签名文件，请重新检查并下载更新。",
        );
      }
      const sources = [
        { filePath: this.updaterPath, label: "更新助手", codePrefix: "launcher.update_helper" },
        { filePath: manifestSource, label: "发布清单", codePrefix: "launcher.update_manifest" },
        { filePath: signatureSource, label: "发布签名", codePrefix: "launcher.update_signature" },
        { filePath: this.downloaded.artifact_path, label: "更新包", codePrefix: "launcher.update_artifact" },
      ];
      await Promise.all(sources.map((source) => assertRegularFile(source.filePath, source.label, source.codePrefix)));
      const cacheRoot = path.join(this.basePath, "cache", "downloads", "updates");
      for (const source of [manifestSource, signatureSource, this.downloaded.artifact_path]) {
        if (!pathInside(cacheRoot, source)) {
          throw new ReleaseUpdateError(
            "launcher.update_input_outside_cache",
            "更新事务输入位置无效",
            "发布清单、签名或更新包位于受控下载缓存之外。",
          );
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
        errorCode: "",
        canCheck: false,
        canDownload: false,
        canInstall: false,
      });
      return this.cached;
    } catch (error) {
      const failure = describeReleaseFailure(error, {
        installRoot: this.basePath,
        updaterPath: this.updaterPath,
        transactionRoot,
      });
      let cleanupFailure: ReleaseFailureDescription | null = null;
      try {
        await fs.rm(transactionRoot, { recursive: true, force: true });
      } catch (cleanupError) {
        cleanupFailure = describeReleaseFailure(cleanupError, {
          installRoot: this.basePath,
          updaterPath: this.updaterPath,
          transactionRoot,
        });
      }
      this.cached = createSnapshot({
        ...this.cached,
        status: "failed",
        summary: failure.summary,
        detail: cleanupFailure
          ? `${failure.detail}\n清理更新事务目录失败（${cleanupFailure.errorCode}）：${cleanupFailure.detail}`
          : failure.detail,
        errorCode: failure.errorCode,
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
      errorCode: "",
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

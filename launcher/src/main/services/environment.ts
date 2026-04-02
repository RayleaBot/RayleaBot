import { execFile } from "node:child_process";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";
import type { EnvironmentCheckResult, EnvironmentInspection, LauncherResolvedSettings } from "../../shared/launcher-models";

const execFileAsync = promisify(execFile);

export interface EnvironmentProbeInput {
  serverExecutableExists: boolean;
  userConfigExists: boolean;
  defaultConfigExists: boolean;
  workdirWritable: boolean;
  depsManifestExists: boolean;
  depsManifestText?: string;
  templatesExist: boolean;
  templatesHaveFiles: boolean;
  platform: string;
  longPaths: "enabled" | "disabled" | "unsupported" | "unknown";
  runtimeResourceStates?: Partial<Record<ManagedRuntimeKind, ManagedRuntimeState>>;
}

type LongPathsStatus = EnvironmentProbeInput["longPaths"];
type ManagedRuntimeKind = "chromium" | "python-runtime" | "nodejs-runtime";
type ExecFileLike = (
  file: string,
  args: string[],
) => Promise<{ stdout: string; stderr: string }>;
type ManagedRuntimeState = {
  metadataComplete: boolean;
  cachedArchivePresent: boolean;
  preparedStorePresent: boolean;
  archivePath?: string;
  storeRoot?: string;
};
type DepsManifestResource = {
  id?: string;
  platform?: string;
  kind?: string;
  version?: string;
  source?: string;
  sha256?: string;
  archive_format?: string;
  entrypoints?: Record<string, string[]>;
};

function normalizeHostPlatform(): string {
  const arch = os.arch();
  switch (process.platform) {
    case "win32":
      return `windows-${arch}`;
    case "linux":
      return `linux-${arch}`;
    case "darwin":
      return `macos-${arch}`;
    default:
      return `${process.platform}-${arch}`;
  }
}

function resolveRuntimeRootFromConfigPath(configPath: string) {
  const absoluteConfigPath = path.resolve(configPath);
  return path.dirname(path.dirname(absoluteConfigPath));
}

function parseDepsManifest(probe: EnvironmentProbeInput): { invalid: boolean; resources: DepsManifestResource[] } {
  if (!probe.depsManifestText) {
    return { invalid: false, resources: [] };
  }
  try {
    const payload = JSON.parse(probe.depsManifestText) as { manifest_version?: number; resources?: DepsManifestResource[] };
    if (payload.manifest_version !== 2) {
      return { invalid: true, resources: [] };
    }
    return { invalid: false, resources: payload.resources ?? [] };
  } catch {
    return { invalid: true, resources: [] };
  }
}

function resourceHasCompleteMetadata(resource?: DepsManifestResource) {
  if (!resource) {
    return false;
  }
  if (!["zip", "tar.gz", "tar.xz"].includes(resource.archive_format?.trim() ?? "")) {
    return false;
  }

  const source = resource.source?.trim() ?? "";
  if (!source.startsWith("https://") || source.toUpperCase().includes("TODO(")) {
    return false;
  }

  const sha256 = resource.sha256?.trim().toLowerCase() ?? "";
  if (sha256.includes("todo(")) {
    return false;
  }

  if (!/^[0-9a-f]{64}$/.test(sha256)) {
    return false;
  }

  return resourceHasRequiredEntrypoints(resource);
}

function findPlatformResource(resources: DepsManifestResource[], platform: string, kind: string) {
  return resources.find((item) => item.platform === platform && item.kind === kind);
}

function resourceHasRequiredEntrypoints(resource: DepsManifestResource) {
  const entrypoints = resource.entrypoints ?? {};
  const requiredKeys = requiredEntrypointKeys(resource.kind);
  if (requiredKeys.length === 0) {
    return false;
  }

  return requiredKeys.every((key) => {
    const candidates = entrypoints[key];
    return Array.isArray(candidates) && candidates.some((candidate) => {
      const value = candidate.trim();
      return value.length > 0 && !value.startsWith("..") && !path.isAbsolute(value);
    });
  });
}

function requiredEntrypointKeys(kind?: string) {
  switch (kind) {
    case "chromium":
      return ["browser"];
    case "python-runtime":
      return ["python", "pip"];
    case "nodejs-runtime":
      return ["node", "npm"];
    default:
      return [];
  }
}

function archiveSuffix(format?: string) {
  switch (format?.trim()) {
    case "tar.gz":
      return ".tar.gz";
    case "tar.xz":
      return ".tar.xz";
    default:
      return ".zip";
  }
}

function runtimeBootstrapRemediation(kind: ManagedRuntimeKind, archivePath?: string, storeRoot?: string) {
  const fallbacks = [archivePath ? `预置已校验归档到 ${archivePath}` : "", storeRoot ? `预展开到 ${storeRoot}` : ""]
    .filter((item) => item.length > 0)
    .join("，或");

  if (kind === "chromium") {
    if (!fallbacks) {
      return "请先准备受控 Chromium 运行时，或在配置中显式设置 render.browser_path。";
    }
    return `请先准备受控 Chromium 运行时，或在配置中显式设置 render.browser_path。离线环境可${fallbacks}`;
  }
  if (!fallbacks) {
    return "请联网准备受控运行时，或按正式目录结构手动预置资源。";
  }
  return `请联网准备受控运行时；离线或受限网络环境可${fallbacks}`;
}

function resolveManagedRuntimeState(
  probe: EnvironmentProbeInput,
  resource: DepsManifestResource | undefined,
  kind: ManagedRuntimeKind,
): ManagedRuntimeState {
  const state = probe.runtimeResourceStates?.[kind];
  if (state) {
    return state;
  }
  return {
    metadataComplete: resourceHasCompleteMetadata(resource),
    cachedArchivePresent: false,
    preparedStorePresent: false,
  };
}

function bootstrapStateIssue(
  code: string,
  title: string,
  kind: ManagedRuntimeKind,
  state: ManagedRuntimeState,
  preparedSummary: string,
  cachedSummary: string,
  onDemandSummary: string,
  warningSummary: string,
  preparedDetail: string,
  cachedDetail: string,
  onDemandDetail: string,
  warningDetail: string,
  metadataRemediation: string,
): EnvironmentCheckResult {
  if (!state.metadataComplete) {
    return {
      code,
      title,
      severity: "warning",
      summary: warningSummary,
      detail: warningDetail,
      remediation: metadataRemediation,
    };
  }
  if (state.preparedStorePresent) {
    return {
      code,
      title,
      severity: "ok",
      summary: preparedSummary,
      detail: preparedDetail,
      remediation: "",
    };
  }
  if (state.cachedArchivePresent) {
    return {
      code,
      title,
      severity: "ok",
      summary: cachedSummary,
      detail: cachedDetail,
      remediation: "",
    };
  }
  return {
    code,
    title,
    severity: "ok",
    summary: onDemandSummary,
    detail: onDemandDetail,
    remediation: runtimeBootstrapRemediation(kind, state.archivePath, state.storeRoot),
  };
}

export async function detectWindowsLongPathsStatus(
  runQuery: ExecFileLike = execFileAsync,
): Promise<LongPathsStatus> {
  try {
    const { stdout } = await runQuery("reg.exe", [
      "query",
      "HKLM\\SYSTEM\\CurrentControlSet\\Control\\FileSystem",
      "/v",
      "LongPathsEnabled",
    ]);

    const longPathsLine = stdout
      .split(/\r?\n/)
      .find((line) => line.includes("LongPathsEnabled"));

    if (!longPathsLine) {
      return "unknown";
    }

    const rawValues = longPathsLine.match(/0x[0-9a-fA-F]+|\d+/g);
    if (!rawValues || rawValues.length === 0) {
      return "unknown";
    }

    const rawValue = rawValues.at(-1) ?? "";
    const parsedValue = rawValue.startsWith("0x")
      ? Number.parseInt(rawValue, 16)
      : Number.parseInt(rawValue, 10);

    if (parsedValue === 1) {
      return "enabled";
    }
    if (parsedValue === 0) {
      return "disabled";
    }
    return "unknown";
  } catch {
    return "unknown";
  }
}

export async function inspectLauncherEnvironment(probe: EnvironmentProbeInput): Promise<EnvironmentInspection> {
  const checks: EnvironmentCheckResult[] = [];

  checks.push(
    probe.serverExecutableExists
      ? {
          code: "server.executable",
          title: "服务端可执行文件",
          severity: "ok",
          summary: "已找到可执行文件。",
          detail: "服务端可执行文件可用。",
          remediation: "",
        }
      : {
          code: "server.executable_missing",
          title: "服务端可执行文件",
          severity: "error",
          summary: "未找到服务端可执行文件。",
          detail: "当前路径下缺少 raylea-server 可执行文件。",
          remediation: "请将启动器设置更新为有效的 raylea-server 可执行文件路径。",
        },
  );

  if (!probe.userConfigExists) {
    checks.push(
      probe.defaultConfigExists
        ? {
            code: "config.bootstrap_available",
            title: "用户配置",
            severity: "warning",
            summary: "首次启动时会自动生成用户配置。",
            detail: "缺少用户配置文件。",
            remediation: "启动服务后会基于 default.yaml 生成首份用户配置。",
          }
        : {
            code: "config.missing",
            title: "配置基线",
            severity: "error",
            summary: "配置基线不完整。",
            detail: "缺少用户配置和默认模板。",
            remediation: "请先恢复 default.yaml 与 user.yaml 所在目录。",
          },
    );
  } else {
    checks.push({
      code: "config.file",
      title: "用户配置",
      severity: "ok",
      summary: "配置文件可读。",
      detail: "用户配置可用。",
      remediation: "",
    });
  }

  checks.push(
    probe.workdirWritable
      ? {
          code: "workdir.ready",
          title: "工作目录",
          severity: "ok",
          summary: "工作目录可写。",
          detail: "工作目录可写。",
          remediation: "",
        }
      : {
          code: "workdir.unwritable",
          title: "工作目录",
          severity: "error",
          summary: "工作目录不可写。",
          detail: "工作目录写入失败。",
          remediation: "请先选择可写的工作目录，再启动服务。",
        },
  );

  if (!probe.depsManifestExists) {
    checks.push({
      code: "deps.manifest_missing",
      title: ".deps 清单",
      severity: "warning",
      summary: "依赖清单缺失。",
      detail: "缺少 manifest.json。",
      remediation: "请先恢复打包后的 .deps 资源。",
    });
  } else {
    const parsedManifest = parseDepsManifest(probe);
    if (parsedManifest.invalid) {
      checks.push({
        code: "deps.manifest_invalid",
        title: ".deps 清单",
        severity: "warning",
        summary: "依赖清单格式无效。",
        detail: "manifest.json 无法解析。",
        remediation: "请重新生成或恢复有效的 .deps 清单。",
      });
    } else {
      const resources = parsedManifest.resources;
      const hasCurrentPlatform = resources.some((item) => item.platform === probe.platform);
      checks.push(
        hasCurrentPlatform
          ? {
              code: "deps.manifest",
              title: ".deps 清单",
              severity: "ok",
              summary: "依赖清单可用。",
              detail: "已包含当前平台资源。",
              remediation: "",
            }
          : {
              code: "deps.manifest_platform_missing",
              title: ".deps 清单",
              severity: "warning",
              summary: "依赖清单缺少当前平台资源。",
              detail: `清单中没有 ${probe.platform} 资源。`,
              remediation: "请为当前平台重新生成或恢复打包后的 .deps 清单。",
          },
      );

      const chromiumResource = findPlatformResource(resources, probe.platform, "chromium");
      checks.push(
        chromiumResource
          ? bootstrapStateIssue(
              "deps.chromium",
              "Chromium 资源",
              "chromium",
              resolveManagedRuntimeState(probe, chromiumResource, "chromium"),
              "Chromium 资源已准备完成。",
              "Chromium 资源归档已缓存，可离线准备。",
              "Chromium 资源可按需准备。",
              "Chromium 资源元数据不完整。",
              "当前平台的受控 Chromium 已展开，可直接用于 render.image。",
              "当前平台的受控 Chromium 归档已缓存，可在离线环境展开。",
              "当前平台的受控 Chromium 元数据完整，可在需要时自动准备。",
              `依赖清单中缺少 ${probe.platform} Chromium 资源的有效 archive_format、entrypoints、source 或 sha256。`,
              "请在 .deps/manifest.json 中补齐当前平台 Chromium 资源的 archive_format、entrypoints、source 与 sha256。",
            )
          : {
              code: "deps.chromium_missing",
              title: "Chromium 资源",
              severity: "warning",
              summary: "缺少 Chromium 资源声明。",
              detail: `依赖清单中没有 ${probe.platform} Chromium 资源。`,
              remediation: "启用 render.image 之前，请先恢复 Chromium 资源。",
            },
      );

      const pythonResource = findPlatformResource(resources, probe.platform, "python-runtime");
      const pythonState = resolveManagedRuntimeState(probe, pythonResource, "python-runtime");
      checks.push(
        resourceHasCompleteMetadata(pythonResource)
          ? {
              code: "deps.python_runtime_metadata",
              title: "Python 运行时元数据",
              severity: "ok",
              summary: "Python 运行时元数据完整。",
              detail: "依赖清单中已包含当前平台 Python 运行时的来源与校验值。",
              remediation: "",
            }
          : {
              code: "deps.python_runtime_metadata_incomplete",
              title: "Python 运行时元数据",
              severity: "warning",
              summary: "Python 运行时元数据不完整。",
              detail: `依赖清单中缺少 ${probe.platform} Python 运行时的有效 source 或 sha256。`,
              remediation: "请在 .deps/manifest.json 中补齐当前平台 Python 运行时的 source 与 sha256。",
            },
      );
      checks.push(
        bootstrapStateIssue(
          "runtime.python_managed_ready",
          "Python 运行时准备",
          "python-runtime",
          pythonState,
          "受控 Python 运行时已准备完成。",
          "受控 Python 运行时归档已缓存，可离线准备。",
          "受控 Python 运行时可按需准备。",
          "受控 Python 运行时当前不可准备。",
          "当前平台的受控 Python 运行时已展开，可直接用于插件依赖安装与运行。",
          "当前平台的受控 Python 运行时归档已缓存，可在离线环境展开。",
          "当前平台的受控 Python 运行时元数据完整，可在需要时自动准备。",
          `当前平台的受控 Python 运行时缺少有效元数据或本地资源。`,
          "请在 .deps/manifest.json 中补齐当前平台 Python 运行时的 archive_format、entrypoints、source 与 sha256。",
        ),
      );

      const nodeResource = findPlatformResource(resources, probe.platform, "nodejs-runtime");
      const nodeState = resolveManagedRuntimeState(probe, nodeResource, "nodejs-runtime");
      checks.push(
        resourceHasCompleteMetadata(nodeResource)
          ? {
              code: "deps.nodejs_runtime_metadata",
              title: "Node.js 运行时元数据",
              severity: "ok",
              summary: "Node.js 运行时元数据完整。",
              detail: "依赖清单中已包含当前平台 Node.js 运行时的来源与校验值。",
              remediation: "",
            }
          : {
              code: "deps.nodejs_runtime_metadata_incomplete",
              title: "Node.js 运行时元数据",
              severity: "warning",
              summary: "Node.js 运行时元数据不完整。",
              detail: `依赖清单中缺少 ${probe.platform} Node.js 运行时的有效 source 或 sha256。`,
              remediation: "请在 .deps/manifest.json 中补齐当前平台 Node.js 运行时的 source 与 sha256。",
            },
      );
      checks.push(
        bootstrapStateIssue(
          "runtime.node_managed_ready",
          "Node.js 运行时准备",
          "nodejs-runtime",
          nodeState,
          "受控 Node.js 运行时已准备完成。",
          "受控 Node.js 运行时归档已缓存，可离线准备。",
          "受控 Node.js 运行时可按需准备。",
          "受控 Node.js 运行时当前不可准备。",
          "当前平台的受控 Node.js 运行时已展开，可直接用于插件依赖安装与运行。",
          "当前平台的受控 Node.js 运行时归档已缓存，可在离线环境展开。",
          "当前平台的受控 Node.js 运行时元数据完整，可在需要时自动准备。",
          `当前平台的受控 Node.js 运行时缺少有效元数据或本地资源。`,
          "请在 .deps/manifest.json 中补齐当前平台 Node.js 运行时的 archive_format、entrypoints、source 与 sha256。",
        ),
      );
      checks.push(
        bootstrapStateIssue(
          "runtime.npm_managed_ready",
          "npm 准备",
          "nodejs-runtime",
          nodeState,
          "受控 npm 已准备完成。",
          "受控 npm 归档已缓存，可离线准备。",
          "受控 npm 可按需准备。",
          "受控 npm 当前不可准备。",
          "当前平台的受控 npm 已展开，可直接用于插件依赖安装。",
          "当前平台的受控 npm 归档已缓存，可在离线环境展开。",
          "当前平台的受控 npm 元数据完整，可在需要时自动准备。",
          `当前平台的受控 npm 缺少有效元数据或本地资源。`,
          "请在 .deps/manifest.json 中补齐当前平台 Node.js 运行时的 archive_format、entrypoints、source 与 sha256。",
        ),
      );
    }
  }

  if (!probe.templatesExist) {
    checks.push({
      code: "render.templates_missing",
      title: "模板资源",
      severity: "warning",
      summary: "模板资源缺失。",
      detail: "缺少模板目录。",
      remediation: "启用 render.image 之前，请先补齐模板资源。",
    });
  } else if (!probe.templatesHaveFiles) {
    checks.push({
      code: "render.templates_empty",
      title: "模板资源",
      severity: "warning",
      summary: "模板资源为空。",
      detail: "模板目录中没有文件。",
      remediation: "启用 render.image 之前，请先补齐模板资源。",
    });
  } else {
    checks.push({
      code: "render.templates",
      title: "模板资源",
      severity: "ok",
      summary: "模板资源可用。",
      detail: "模板资源可用。",
      remediation: "",
    });
  }

  switch (probe.longPaths) {
    case "enabled":
      checks.push({
        code: "os.long_paths_enabled",
        title: "长路径支持",
        severity: "ok",
        summary: "已启用长路径支持。",
        detail: "长路径支持已开启。",
        remediation: "",
      });
      break;
    case "disabled":
      checks.push({
        code: "os.long_paths_disabled",
        title: "长路径支持",
        severity: "warning",
        summary: "长路径支持未启用。",
        detail: "当前系统未启用长路径支持。",
        remediation: "建议启用长路径支持以减少资源展开失败。",
      });
      break;
    case "unknown":
      checks.push({
        code: "os.long_paths_unknown",
        title: "长路径支持",
        severity: "warning",
        summary: "无法确认长路径支持状态。",
        detail: "当前无法判断长路径支持状态。",
        remediation: "若资源展开遇到路径限制，请手动检查长路径设置。",
      });
      break;
    default:
      checks.push({
        code: "os.long_paths_unavailable",
        title: "长路径支持",
        severity: "ok",
        summary: "当前平台无需额外处理。",
        detail: "当前平台不使用 Windows 长路径注册表检查。",
        remediation: "",
      });
      break;
  }

  return {
    checks,
    hasBlockingIssues: checks.some((item) => item.severity === "error"),
    canBootstrapUserConfig: checks.some((item) => item.code === "config.bootstrap_available"),
  };
}

async function pathExists(targetPath: string) {
  try {
    await fs.access(targetPath);
    return true;
  } catch {
    return false;
  }
}

async function directoryHasFiles(targetPath: string) {
  try {
    const entries = await fs.readdir(targetPath);
    return entries.length > 0;
  } catch {
    return false;
  }
}

async function fileExists(targetPath: string) {
  try {
    const stat = await fs.stat(targetPath);
    return stat.isFile();
  } catch {
    return false;
  }
}

async function preparedEntrypointExists(storeRoot: string, candidates: string[] | undefined) {
  if (!Array.isArray(candidates) || candidates.length === 0) {
    return false;
  }
  for (const candidate of candidates) {
    const value = candidate.trim();
    if (!value || value.startsWith("..") || path.isAbsolute(value)) {
      continue;
    }
    if (await fileExists(path.join(storeRoot, ...value.split("/")))) {
      return true;
    }
  }
  return false;
}

async function collectManagedRuntimeStates(
  runtimeRoot: string,
  resources: DepsManifestResource[],
  platform: string,
): Promise<Partial<Record<ManagedRuntimeKind, ManagedRuntimeState>>> {
  const states: Partial<Record<ManagedRuntimeKind, ManagedRuntimeState>> = {};
  for (const kind of ["chromium", "python-runtime", "nodejs-runtime"] as ManagedRuntimeKind[]) {
    const resource = findPlatformResource(resources, platform, kind);
    if (!resource || !resource.id || !resource.version) {
      continue;
    }
    const archivePath = path.join(
      runtimeRoot,
      "cache",
      "downloads",
      "runtime",
      `${resource.id}-${resource.version}${archiveSuffix(resource.archive_format)}`,
    );
    const storeRoot = path.join(runtimeRoot, ".deps", "store", resource.id, resource.version);
    const requiredKeys = requiredEntrypointKeys(resource.kind);
    const preparedChecks = await Promise.all(
      requiredKeys.map(async (key) => preparedEntrypointExists(storeRoot, resource.entrypoints?.[key])),
    );
    states[kind] = {
      metadataComplete: resourceHasCompleteMetadata(resource),
      cachedArchivePresent: await fileExists(archivePath),
      preparedStorePresent: requiredKeys.length > 0 && preparedChecks.every(Boolean),
      archivePath,
      storeRoot,
    };
  }
  return states;
}

async function isWorkdirWritable(targetPath: string) {
  try {
    await fs.mkdir(targetPath, { recursive: true });
    const probePath = path.join(targetPath, ".launcher-write-test");
    await fs.writeFile(probePath, "ok", "utf8");
    await fs.rm(probePath, { force: true });
    return true;
  } catch {
    return false;
  }
}

export async function inspectEnvironmentFromNode(settings: LauncherResolvedSettings): Promise<EnvironmentInspection> {
  const workdir = settings.workdir;
  const runtimeRoot = resolveRuntimeRootFromConfigPath(settings.configPath);
  const depsManifestPath = path.join(runtimeRoot, ".deps", "manifest.json");
  const templatesPath = path.join(runtimeRoot, "templates");
  const defaultConfigPath = path.join(path.dirname(settings.configPath), "default.yaml");

  let depsManifestText: string | undefined;
  let runtimeResourceStates: Partial<Record<ManagedRuntimeKind, ManagedRuntimeState>> | undefined;
  if (await pathExists(depsManifestPath)) {
    depsManifestText = await fs.readFile(depsManifestPath, "utf8");
    const parsedManifest = parseDepsManifest({
      serverExecutableExists: true,
      userConfigExists: true,
      defaultConfigExists: true,
      workdirWritable: true,
      depsManifestExists: true,
      depsManifestText,
      templatesExist: true,
      templatesHaveFiles: true,
      platform: normalizeHostPlatform(),
      longPaths: "unknown",
    });
    if (!parsedManifest.invalid) {
      runtimeResourceStates = await collectManagedRuntimeStates(runtimeRoot, parsedManifest.resources, normalizeHostPlatform());
    }
  }

  return inspectLauncherEnvironment({
    serverExecutableExists: await pathExists(settings.serverExecutablePath),
    userConfigExists: await pathExists(settings.configPath),
    defaultConfigExists: await pathExists(defaultConfigPath),
    workdirWritable: await isWorkdirWritable(workdir),
    depsManifestExists: Boolean(depsManifestText),
    depsManifestText,
    templatesExist: await pathExists(templatesPath),
    templatesHaveFiles: await directoryHasFiles(templatesPath),
    platform: normalizeHostPlatform(),
    longPaths: process.platform === "win32" ? await detectWindowsLongPathsStatus() : "unsupported",
    runtimeResourceStates,
  });
}

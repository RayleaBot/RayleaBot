import fs from "node:fs/promises";
import path from "node:path";
import type { EnvironmentCheckResult, EnvironmentInspection, LauncherResolvedSettings } from "../../shared/launcher-models";
import { fileExists, pathExists } from "./fs-utils";

export interface EnvironmentProbeInput {
  installationRootExists?: boolean;
  launcherSettingsResolved?: boolean;
  serverExecutableExists: boolean;
  userConfigExists: boolean;
  defaultConfigExists: boolean;
  workdirWritable: boolean;

  depsManifestExists?: boolean;
  depsManifestText?: string;
  depsManifestPath?: string;
  templatesExist?: boolean;
  templatesHaveFiles?: boolean;
  platform?: string;
  longPaths?: string;
  runtimeResourceStates?: Record<string, RuntimeResourceState>;
}

type EnvironmentCheckDraft = Omit<EnvironmentCheckResult, "scope">;

type RuntimeResourceKind = "chromium" | "python-runtime" | "nodejs-runtime";

type RuntimeResourceState = {
  archivePath: string;
  archiveExists: boolean;
  storeRoot: string;
  storeRootExists: boolean;
  tempRootPaths: string[];
  preparedStorePresent: boolean;
  missingEntrypoints: string[];
  primaryEntrypoint: string;
};

type DepsManifestResource = {
  id?: unknown;
  kind?: unknown;
  version?: unknown;
  platform?: unknown;
  sources?: unknown;
  sha256?: unknown;
  archive_format?: unknown;
  entrypoints?: unknown;
};

type RuntimeResourceDefinition = {
  kind: RuntimeResourceKind;
  title: string;
  label: string;
  requiredEntrypoints: string[];
  readyCode: string;
  missingCode: string;
  metadataCode: string;
  notReadyCode: string;
  partialCode: string;
  tempCode: string;
};

const runtimeResourceDefinitions: RuntimeResourceDefinition[] = [
  {
    kind: "chromium",
    title: "Chromium 依赖",
    label: "Chromium 依赖",
    requiredEntrypoints: ["browser"],
    readyCode: "chromium.ready",
    missingCode: "chromium.resource_missing",
    metadataCode: "chromium.metadata_incomplete",
    notReadyCode: "chromium.not_ready",
    partialCode: "chromium.entrypoint_missing",
    tempCode: "chromium.extract_incomplete",
  },
  {
    kind: "python-runtime",
    title: "Python 依赖",
    label: "Python 依赖",
    requiredEntrypoints: ["python"],
    readyCode: "python.ready",
    missingCode: "python.resource_missing",
    metadataCode: "python.metadata_incomplete",
    notReadyCode: "python.not_ready",
    partialCode: "python.entrypoint_missing",
    tempCode: "python.extract_incomplete",
  },
  {
    kind: "nodejs-runtime",
    title: "Node.js / npm 依赖",
    label: "Node.js / npm 依赖",
    requiredEntrypoints: ["node", "npm"],
    readyCode: "nodejs.ready",
    missingCode: "nodejs.resource_missing",
    metadataCode: "nodejs.metadata_incomplete",
    notReadyCode: "nodejs.not_ready",
    partialCode: "nodejs.entrypoint_missing",
    tempCode: "nodejs.extract_incomplete",
  },
];

function withScope(check: EnvironmentCheckDraft): EnvironmentCheckResult {
  return {
    ...check,
    scope: "preflight",
  };
}

export async function inspectLauncherEnvironment(probe: EnvironmentProbeInput): Promise<EnvironmentInspection> {
  const checks: EnvironmentCheckResult[] = [];

  checks.push(
    withScope(
      probe.installationRootExists === false
        ? {
            code: "launcher.installation_root_missing",
            title: "安装目录",
            severity: "error",
            summary: "安装目录不可用。",
            detail: "当前安装目录不存在或无法访问。",
            remediation: "请先选择有效的 RayleaBot 安装目录。",
          }
        : {
            code: "launcher.installation_root",
            title: "安装目录",
            severity: "ok",
            summary: "安装目录可用。",
            detail: "安装目录可访问。",
            remediation: "",
          },
    ),
  );

  checks.push(
    withScope(
      probe.launcherSettingsResolved === false
        ? {
            code: "launcher.settings_invalid",
            title: "启动器设置",
            severity: "error",
            summary: "启动器设置无效。",
            detail: "当前设置无法解析为有效的服务端路径和工作目录。",
            remediation: "请检查安装目录和高级覆盖路径。",
          }
        : {
            code: "launcher.settings",
            title: "启动器设置",
            severity: "ok",
            summary: "启动器设置可用。",
            detail: "当前设置已解析。",
            remediation: "",
          },
    ),
  );

  checks.push(
    withScope(
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
    ),
  );

  checks.push(
    withScope(
      !probe.userConfigExists
        ? probe.defaultConfigExists
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
              title: "用户配置",
              severity: "error",
              summary: "配置基线不完整。",
              detail: "缺少 user.yaml，且当前目录下没有可用的 default.yaml。",
              remediation: "请先恢复 config/default.yaml 与 config/user.yaml 所在目录。",
            }
        : {
            code: "config.file",
            title: "用户配置",
            severity: "ok",
            summary: "配置文件可读。",
            detail: "用户配置可用。",
            remediation: "",
          },
    ),
  );

  checks.push(
    withScope(
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
    ),
  );

  checks.push(...inspectDepsChecks(probe));

  return {
    checks,
    preflightChecks: checks,
    advisoryChecks: [],
    hasBlockingIssues: checks.some((item) => item.severity === "error"),
    canBootstrapUserConfig: checks.some((item) => item.code === "config.bootstrap_available"),
  };
}

function inspectDepsChecks(probe: EnvironmentProbeInput): EnvironmentCheckResult[] {
  if (probe.depsManifestExists === undefined && probe.depsManifestText === undefined) {
    return [];
  }

  const manifestPath = probe.depsManifestPath?.trim() || ".deps/manifest.json";
  if (probe.depsManifestExists === false) {
    return [
      withScope({
        code: "deps.manifest_missing",
        title: "运行环境清单",
        severity: "warning",
        summary: ".deps/manifest.json 未找到。",
        detail: `检查路径：${manifestPath}`,
        remediation: "请恢复 .deps/manifest.json。",
      }),
    ];
  }

  const manifestText = probe.depsManifestText?.trim() ?? "";
  if (manifestText === "") {
    return [
      withScope({
        code: "deps.manifest_missing",
        title: "运行环境清单",
        severity: "warning",
        summary: ".deps/manifest.json 未找到。",
        detail: `检查路径：${manifestPath}`,
        remediation: "请恢复 .deps/manifest.json。",
      }),
    ];
  }

  const manifest = parseDepsManifest(manifestText);
  if (!manifest.ok) {
    return [
      withScope({
        code: "deps.manifest_invalid",
        title: "运行环境清单",
        severity: "warning",
        summary: ".deps/manifest.json 内容无效。",
        detail: `检查路径：${manifestPath}`,
        remediation: "请恢复有效的 .deps/manifest.json。",
      }),
    ];
  }

  const platform = probe.platform?.trim() || currentManifestPlatform();
  const resources = manifest.resources;
  const platformResources = resources.filter((resource) => stringValue(resource.platform) === platform);
  const checks: EnvironmentCheckResult[] = [
    withScope(
      platformResources.length > 0
        ? {
            code: "deps.manifest",
            title: "运行环境清单",
            severity: "ok",
            summary: ".deps/manifest.json 已包含当前平台资源。",
            detail: `平台：${platform}`,
            remediation: "",
          }
        : {
            code: "deps.manifest_platform_missing",
            title: "运行环境清单",
            severity: "warning",
            summary: ".deps/manifest.json 缺少当前平台资源。",
            detail: `平台：${platform}。检查路径：${manifestPath}`,
            remediation: "请恢复包含当前平台资源的 .deps/manifest.json。",
          },
    ),
  ];

  for (const definition of runtimeResourceDefinitions) {
    const resource = platformResources.find((item) => stringValue(item.kind) === definition.kind);
    checks.push(inspectRuntimeResource(definition, resource, probe.runtimeResourceStates?.[definition.kind]));
  }

  return checks;
}

function parseDepsManifest(payload: string): { ok: true; resources: DepsManifestResource[] } | { ok: false } {
  try {
    const parsed = JSON.parse(payload) as { manifest_version?: unknown; resources?: unknown };
    if (parsed.manifest_version !== 3 || !Array.isArray(parsed.resources)) {
      return { ok: false };
    }
    return { ok: true, resources: parsed.resources as DepsManifestResource[] };
  } catch {
    return { ok: false };
  }
}

function inspectRuntimeResource(
  definition: RuntimeResourceDefinition,
  resource: DepsManifestResource | undefined,
  state: RuntimeResourceState | undefined,
): EnvironmentCheckResult {
  if (!resource) {
    return withScope({
      code: definition.missingCode,
      title: definition.title,
      severity: "warning",
      summary: "未写入 .deps/manifest.json。",
      detail: "",
      remediation: `请恢复 .deps/manifest.json 中的 ${definition.label}资源。`,
    });
  }

  if (!runtimeResourceMetadataComplete(resource, definition.requiredEntrypoints)) {
    return withScope({
      code: definition.metadataCode,
      title: definition.title,
      severity: "warning",
      summary: "清单不完整。",
      detail: ".deps/manifest.json 中缺少来源、校验值、安装包格式或入口文件。",
      remediation: `请恢复 .deps/manifest.json 中当前平台的 ${definition.label}资源。`,
    });
  }

  if (!state) {
    return withScope({
      code: definition.notReadyCode,
      title: definition.title,
      severity: "warning",
      summary: "状态未检查。",
      detail: "",
      remediation: "请刷新环境检查。",
    });
  }

  if (state.preparedStorePresent) {
    return withScope({
      code: definition.readyCode,
      title: definition.title,
      severity: "ok",
      summary: "已解压。",
      detail: state.primaryEntrypoint ? `入口位置：${state.primaryEntrypoint}` : `解压位置：${state.storeRoot}`,
      remediation: "",
    });
  }

  if (state.tempRootPaths.length > 0 && !state.storeRootExists) {
    return withScope({
      code: definition.tempCode,
      title: definition.title,
      severity: "warning",
      summary: "上次解压未完成。",
      detail: `下载位置：${state.archivePath}。解压位置：${state.storeRoot}。临时目录：${state.tempRootPaths.join("、")}`,
      remediation: `启动运行环境任务重新准备 ${definition.label}。`,
    });
  }

  if (state.missingEntrypoints.length > 0 && state.storeRootExists) {
    return withScope({
      code: definition.partialCode,
      title: definition.title,
      severity: "warning",
      summary: "已解压，但入口文件缺失。",
      detail: `缺少：${state.missingEntrypoints.join("、")}。解压位置：${state.storeRoot}`,
      remediation: `启动运行环境任务重新准备 ${definition.label}。`,
    });
  }

  if (state.archiveExists) {
    return withScope({
      code: definition.notReadyCode,
      title: definition.title,
      severity: "warning",
      summary: "已下载，未解压。",
      detail: `下载位置：${state.archivePath}。解压位置：${state.storeRoot}`,
      remediation: `启动运行环境任务解压 ${definition.label}。`,
    });
  }

  return withScope({
    code: definition.notReadyCode,
    title: definition.title,
    severity: "warning",
    summary: "未准备。",
    detail: `下载位置：${state.archivePath}。解压位置：${state.storeRoot}`,
    remediation: `启动运行环境任务下载并解压 ${definition.label}。`,
  });
}

function runtimeResourceMetadataComplete(resource: DepsManifestResource, requiredEntrypoints: string[]) {
  const id = stringValue(resource.id);
  const version = stringValue(resource.version);
  const archiveFormat = stringValue(resource.archive_format);
  const sha256 = stringValue(resource.sha256);
  if (!id || !version || !supportedArchiveFormat(archiveFormat) || !/^[0-9a-f]{64}$/i.test(sha256)) {
    return false;
  }
  if (!Array.isArray(resource.sources) || resource.sources.length === 0 || !resource.sources.every(sourceComplete)) {
    return false;
  }
  if (!entrypointsComplete(resource.entrypoints, requiredEntrypoints)) {
    return false;
  }
  return true;
}

function sourceComplete(source: unknown) {
  if (!source || typeof source !== "object") {
    return false;
  }
  const entry = source as { url?: unknown; kind?: unknown };
  const rawUrl = stringValue(entry.url);
  if (!rawUrl.startsWith("https://")) {
    return false;
  }
  const kind = stringValue(entry.kind);
  return kind === "upstream" || kind === "mirror";
}

function entrypointsComplete(entrypoints: unknown, requiredEntrypoints: string[]) {
  if (!entrypoints || typeof entrypoints !== "object") {
    return false;
  }
  const values = entrypoints as Record<string, unknown>;
  return requiredEntrypoints.every((key) => {
    const candidates = values[key];
    return Array.isArray(candidates) && candidates.some(validEntrypointCandidate);
  });
}

function validEntrypointCandidate(candidate: unknown) {
  const value = stringValue(candidate);
  if (!value || path.isAbsolute(value) || /^[A-Za-z]:/.test(value) || value.startsWith("/") || value.startsWith("\\")) {
    return false;
  }
  return !value.split(/[\\/]+/).some((segment) => segment === "..");
}

function supportedArchiveFormat(format: string) {
  return format === "zip" || format === "tar.gz" || format === "tar.xz";
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
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
  const configPath = path.resolve(settings.configPath);
  const defaultConfigPath = path.join(path.dirname(configPath), "default.yaml");
  const depsManifestPath = path.join(settings.installationRoot, ".deps", "manifest.json");
  const depsManifest = await readTextIfExists(depsManifestPath);
  const runtimeResourceStates = depsManifest.exists && depsManifest.text.trim()
    ? await inspectRuntimeResourceStates(settings.installationRoot, depsManifest.text, currentManifestPlatform())
    : {};

  return inspectLauncherEnvironment({
    installationRootExists: await pathExists(settings.installationRoot),
    launcherSettingsResolved: true,
    serverExecutableExists: await pathExists(settings.serverExecutablePath),
    userConfigExists: await pathExists(settings.configPath),
    defaultConfigExists: await pathExists(defaultConfigPath),
    workdirWritable: await isWorkdirWritable(settings.workdir),
    depsManifestExists: depsManifest.exists,
    depsManifestText: depsManifest.text,
    depsManifestPath,
    platform: currentManifestPlatform(),
    runtimeResourceStates,
  });
}

async function readTextIfExists(filePath: string) {
  try {
    return { exists: true, text: await fs.readFile(filePath, "utf8") };
  } catch {
    return { exists: false, text: "" };
  }
}

async function inspectRuntimeResourceStates(
  installationRoot: string,
  manifestText: string,
  platform: string,
): Promise<Record<string, RuntimeResourceState>> {
  const manifest = parseDepsManifest(manifestText);
  if (!manifest.ok) {
    return {};
  }
  const states: Record<string, RuntimeResourceState> = {};
  for (const definition of runtimeResourceDefinitions) {
    const resource = manifest.resources.find(
      (item) => stringValue(item.platform) === platform && stringValue(item.kind) === definition.kind,
    );
    if (!resource || !runtimeResourceMetadataComplete(resource, definition.requiredEntrypoints)) {
      continue;
    }
    states[definition.kind] = await inspectRuntimeResourceState(installationRoot, resource, definition.requiredEntrypoints);
  }
  return states;
}

async function inspectRuntimeResourceState(
  installationRoot: string,
  resource: DepsManifestResource,
  requiredEntrypoints: string[],
): Promise<RuntimeResourceState> {
  const id = stringValue(resource.id);
  const version = stringValue(resource.version);
  const archiveFormat = stringValue(resource.archive_format);
  const archivePath = path.join(installationRoot, "cache", "downloads", "runtime", `${id}-${version}${archiveSuffix(archiveFormat)}`);
  const storeRoot = path.join(installationRoot, ".deps", "store", id, version);
  const tempRootPaths = await findRuntimeTempRoots(path.dirname(storeRoot), id, version);
  const entrypoints = resource.entrypoints as Record<string, unknown>;
  const missingEntrypoints: string[] = [];
  let primaryEntrypoint = "";

  for (const key of requiredEntrypoints) {
    const candidates = Array.isArray(entrypoints[key]) ? entrypoints[key] : [];
    let resolvedPath = "";
    for (const candidate of candidates) {
      const relative = stringValue(candidate);
      if (!validEntrypointCandidate(relative)) {
        continue;
      }
      const candidatePath = path.join(storeRoot, ...relative.split(/[\\/]+/));
      if (await fileExists(candidatePath)) {
        resolvedPath = candidatePath;
        break;
      }
    }
    if (resolvedPath) {
      primaryEntrypoint ||= resolvedPath;
    } else {
      missingEntrypoints.push(key);
    }
  }

  return {
    archivePath,
    archiveExists: await fileExists(archivePath),
    storeRoot,
    storeRootExists: await pathExists(storeRoot),
    tempRootPaths,
    preparedStorePresent: missingEntrypoints.length === 0,
    missingEntrypoints,
    primaryEntrypoint,
  };
}

async function findRuntimeTempRoots(parent: string, id: string, version: string) {
  try {
    const entries = await fs.readdir(parent, { withFileTypes: true });
    const prefix = `.${id}-${version}-`;
    return entries
      .filter((entry) => entry.isDirectory() && entry.name.startsWith(prefix))
      .map((entry) => path.join(parent, entry.name))
      .sort();
  } catch {
    return [];
  }
}

function archiveSuffix(format: string) {
  switch (format) {
    case "tar.gz":
      return ".tar.gz";
    case "tar.xz":
      return ".tar.xz";
    default:
      return ".zip";
  }
}

function currentManifestPlatform() {
  const platform = process.platform === "win32" ? "windows" : process.platform === "darwin" ? "macos" : process.platform;
  const arch = process.arch === "x64" ? "x64" : process.arch;
  return `${platform}-${arch}`;
}

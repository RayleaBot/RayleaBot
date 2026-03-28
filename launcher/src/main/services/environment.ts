import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import type { EnvironmentCheckResult, EnvironmentInspection, LauncherSettings } from "../../shared/launcher-models";

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
}

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

function parseDepsManifest(probe: EnvironmentProbeInput): Array<{ platform?: string; kind?: string }> {
  if (!probe.depsManifestText) {
    return [];
  }
  try {
    const payload = JSON.parse(probe.depsManifestText) as { resources?: Array<{ platform?: string; kind?: string }> };
    return payload.resources ?? [];
  } catch {
    return [];
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
    const resources = parseDepsManifest(probe);
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

    const hasChromium = resources.some((item) => item.platform === probe.platform && item.kind === "chromium");
    checks.push(
      hasChromium
        ? {
            code: "deps.chromium",
            title: "Chromium 资源",
            severity: "ok",
            summary: "已声明 Chromium 资源。",
            detail: "依赖清单中已包含当前平台 Chromium 资源。",
            remediation: "",
          }
        : {
            code: "deps.chromium_missing",
            title: "Chromium 资源",
            severity: "warning",
            summary: "缺少 Chromium 资源声明。",
            detail: `依赖清单中没有 ${probe.platform} Chromium 资源。`,
            remediation: "启用 render.image 之前，请先恢复 Chromium 资源。",
          },
    );
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

export async function inspectEnvironmentFromNode(settings: LauncherSettings): Promise<EnvironmentInspection> {
  const workdir = settings.workdir;
  const depsManifestPath = path.join(workdir, ".deps", "manifest.json");
  const templatesPath = path.join(workdir, "templates");
  const defaultConfigPath = path.join(path.dirname(settings.configPath), "default.yaml");

  let depsManifestText: string | undefined;
  if (await pathExists(depsManifestPath)) {
    depsManifestText = await fs.readFile(depsManifestPath, "utf8");
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
    longPaths: process.platform === "win32" ? "unknown" : "unsupported",
  });
}

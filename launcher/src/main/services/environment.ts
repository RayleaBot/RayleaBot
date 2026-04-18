import fs from "node:fs/promises";
import path from "node:path";
import type { EnvironmentCheckResult, EnvironmentInspection, LauncherResolvedSettings } from "../../shared/launcher-models";
import { pathExists } from "./fs-utils";

export interface EnvironmentProbeInput {
  installationRootExists?: boolean;
  launcherSettingsResolved?: boolean;
  serverExecutableExists: boolean;
  userConfigExists: boolean;
  defaultConfigExists: boolean;
  workdirWritable: boolean;

  // Legacy fields kept as ignored input so older tests and callers can pass richer
  // probe objects without widening the current local-preflight surface again.
  depsManifestExists?: boolean;
  depsManifestText?: string;
  templatesExist?: boolean;
  templatesHaveFiles?: boolean;
  platform?: string;
  longPaths?: string;
  runtimeResourceStates?: Record<string, unknown>;
}

type EnvironmentCheckDraft = Omit<EnvironmentCheckResult, "scope">;

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

  return {
    checks,
    preflightChecks: checks,
    advisoryChecks: [],
    hasBlockingIssues: checks.some((item) => item.severity === "error"),
    canBootstrapUserConfig: checks.some((item) => item.code === "config.bootstrap_available"),
  };
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

  return inspectLauncherEnvironment({
    installationRootExists: await pathExists(settings.installationRoot),
    launcherSettingsResolved: true,
    serverExecutableExists: await pathExists(settings.serverExecutablePath),
    userConfigExists: await pathExists(settings.configPath),
    defaultConfigExists: await pathExists(defaultConfigPath),
    workdirWritable: await isWorkdirWritable(settings.workdir),
  });
}

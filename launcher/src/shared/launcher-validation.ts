import type { LauncherAdvancedOverrides, LauncherCloseBehavior, LauncherSettings } from "./launcher-models";

const CLOSE_BEHAVIORS = new Set<LauncherCloseBehavior>([
  "ask_every_time",
  "hide_to_tray",
  "exit_application",
]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function readRequiredString(value: unknown) {
  if (typeof value !== "string" || !value.trim()) {
    throw new Error("启动器设置格式无效。");
  }
  return value;
}

function readOptionalString(value: unknown) {
  if (value === undefined) {
    return undefined;
  }
  if (typeof value !== "string") {
    throw new Error("启动器设置格式无效。");
  }
  return value;
}

export function parseLauncherSettingsInput(value: unknown): LauncherSettings {
  if (!isRecord(value)) {
    throw new Error("启动器设置格式无效。");
  }

  const installationRoot = readRequiredString(value.installationRoot);
  const closeBehavior = value.closeBehavior;
  if (!CLOSE_BEHAVIORS.has(closeBehavior as LauncherCloseBehavior)) {
    throw new Error("启动器设置格式无效。");
  }
  const normalizedCloseBehavior = closeBehavior as LauncherCloseBehavior;

  let advancedOverrides: LauncherAdvancedOverrides | undefined;
  if (value.advancedOverrides !== undefined) {
    if (!isRecord(value.advancedOverrides)) {
      throw new Error("启动器设置格式无效。");
    }

    const overrides = {
      serverExecutablePath: readOptionalString(value.advancedOverrides.serverExecutablePath),
      configPath: readOptionalString(value.advancedOverrides.configPath),
      workdir: readOptionalString(value.advancedOverrides.workdir),
    } satisfies LauncherAdvancedOverrides;

    if (overrides.serverExecutablePath || overrides.configPath || overrides.workdir) {
      advancedOverrides = overrides;
    }
  }

  return {
    installationRoot,
    closeBehavior: normalizedCloseBehavior,
    advancedOverrides,
  };
}

export function parseRuntimeBootstrapResources(value: unknown): string[] | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!Array.isArray(value) || value.some((item) => typeof item !== "string" || !item.trim())) {
    throw new Error("运行环境资源列表格式无效。");
  }
  return value;
}

export function sanitizeLauncherWebTargetPath(value: unknown): string {
  if (value === undefined || value === null || value === "") {
    return "";
  }
  if (typeof value !== "string") {
    throw new Error("启动器只允许打开管理界面的相对路径。");
  }

  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }

  const pathCandidate = trimmed.startsWith("/") ? trimmed.slice(1) : trimmed;
  const rawPath = pathCandidate.split(/[?#]/, 1)[0] ?? "";
  if (/^\/|(^|\/)\.\.?(\/|$)|\\/.test(rawPath)) {
    throw new Error("启动器只允许打开管理界面的相对路径。");
  }

  const base = new URL("http://rayleabot.local/");
  const candidate = new URL(pathCandidate || "", base);
  if (candidate.origin !== base.origin || candidate.protocol !== base.protocol) {
    throw new Error("启动器只允许打开管理界面的相对路径。");
  }

  return `${candidate.pathname.replace(/^\//, "")}${candidate.search}${candidate.hash}`;
}

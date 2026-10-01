import {
  ArrowSync24Filled,
  Checkmark24Filled,
  CheckmarkCircle20Regular,
  Dismiss24Filled,
  DocumentText20Regular,
  HeartPulse20Regular,
  Important24Filled,
  Info20Regular,
  PersonAdd24Filled,
  Power24Filled,
  Settings20Regular,
  Status20Regular,
  Warning20Regular,
} from "@fluentui/react-icons";
import type { ReactNode } from "react";

import { getLauncherStateLabel, type LauncherPresentationState } from "@shared/launcher-presentation";
import type { LauncherSettings, ReleaseCheckSnapshot } from "@shared/launcher-models";

export type SectionId = "status" | "environment" | "diagnostics" | "settings" | "about";
export type LauncherVisualTone = "neutral" | "info" | "success" | "attention" | "warning" | "danger";

export const serviceStateConfig: Record<LauncherPresentationState, { label: string; tone: LauncherVisualTone }> = {
  stopped: { label: getLauncherStateLabel("stopped"), tone: "neutral" },
  starting: { label: getLauncherStateLabel("starting"), tone: "info" },
  running: { label: getLauncherStateLabel("running"), tone: "success" },
  setup_required: { label: getLauncherStateLabel("setup_required"), tone: "info" },
  degraded: { label: getLauncherStateLabel("degraded"), tone: "warning" },
  unhealthy: { label: getLauncherStateLabel("unhealthy"), tone: "danger" },
  stopping: { label: getLauncherStateLabel("stopping"), tone: "info" },
  failed: { label: getLauncherStateLabel("failed"), tone: "danger" },
};

/** State glyphs shown in the status lens, so each service state reads without its color. */
export const serviceStateGlyphs: Record<LauncherPresentationState, ReactNode> = {
  stopped: <Power24Filled />,
  starting: <ArrowSync24Filled />,
  running: <Checkmark24Filled />,
  setup_required: <PersonAdd24Filled />,
  degraded: <Important24Filled />,
  unhealthy: <Dismiss24Filled />,
  stopping: <ArrowSync24Filled />,
  failed: <Dismiss24Filled />,
};

export const severityConfig = {
  error: { label: "阻塞", icon: <Warning20Regular /> },
  warning: { label: "警告", icon: <Warning20Regular /> },
  ok: { label: "正常", icon: <CheckmarkCircle20Regular /> },
};

export const sections = [
  { id: "status" as SectionId, title: "运行状态", icon: <Status20Regular /> },
  { id: "environment" as SectionId, title: "环境检查", icon: <HeartPulse20Regular /> },
  { id: "diagnostics" as SectionId, title: "日志诊断", icon: <DocumentText20Regular /> },
  { id: "settings" as SectionId, title: "偏好设置", icon: <Settings20Regular /> },
  { id: "about" as SectionId, title: "关于应用", icon: <Info20Regular /> },
];

export const sectionContent = {
  status: {
    eyebrow: "Service Console",
    title: "运行状态",
  },
  environment: {
    eyebrow: "Environment Review",
    title: "环境检查",
  },
  diagnostics: {
    eyebrow: "Diagnostics",
    title: "日志诊断",
  },
  settings: {
    eyebrow: "Launcher Settings",
    title: "偏好设置",
  },
  about: {
    eyebrow: "About RayleaBot",
    title: "关于应用",
  },
} satisfies Record<SectionId, { eyebrow: string; title: string }>;

const severityOrder = {
  error: 0,
  warning: 1,
  ok: 2,
} satisfies Record<"error" | "warning" | "ok", number>;

export const busyActionLabels: Record<string, string> = {
  refresh: "正在刷新状态",
  start: "正在启动服务",
  stop: "正在停止服务",
  restart: "正在重启服务",
  save: "正在保存设置",
  "open-web": "正在打开管理界面",
  "check-updates": "正在检查更新",
  "apply-update": "正在开始更新",
  "open-repository-page": "正在打开 GitHub",
  "open-release-page": "正在打开发布页",
  "open-logs": "正在打开日志目录",
  "choose-path": "正在选择路径",
  "reset-admin": "正在重置管理员账号",
};

export const closeBehaviorOptions: Array<{
  value: LauncherSettings["closeBehavior"];
  label: string;
  detail: string;
}> = [
  { value: "ask_every_time", label: "每次询问", detail: "每次关闭窗口时都显示确认选项。" },
  { value: "hide_to_tray", label: "隐藏到托盘", detail: "关闭窗口后启动器留在托盘，服务继续运行。" },
  { value: "exit_application", label: "完全退出", detail: "关闭窗口时退出启动器；由启动器启动的服务会一并停止。" },
];

/** A build without build_info.json has no version; while the first check runs the version may still arrive. */
export function formatReleaseVersion({ currentVersion, status }: Pick<ReleaseCheckSnapshot, "currentVersion" | "status">): string {
  return currentVersion.trim() || (status === "checking" ? "读取中" : "开发");
}

const runtimePreparationPrefixes = ["deps.", "chromium.", "ffmpeg."];

export function isRuntimePreparationIssue(code: string): boolean {
  return runtimePreparationPrefixes.some((prefix) => code.startsWith(prefix));
}

export function formatByteCount(value: number | null | undefined): string {
  if (!value || value <= 0) {
    return "";
  }
  const units = ["B", "KB", "MB", "GB"];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  return `${size.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

export function sortChecks<T extends { severity: "ok" | "warning" | "error"; title: string }>(items: T[]): T[] {
  return [...items].sort((left, right) => {
    const severityGap = severityOrder[left.severity] - severityOrder[right.severity];
    if (severityGap !== 0) {
      return severityGap;
    }

    return left.title.localeCompare(right.title, "zh-CN");
  });
}

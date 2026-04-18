import type { PresenceBadgeStatus } from "@fluentui/react-components";
import {
  CheckmarkCircle20Filled,
  DocumentText20Filled,
  HeartPulse20Filled,
  Settings20Filled,
  Status20Filled,
  Warning20Filled,
} from "@fluentui/react-icons";

import { getLauncherStateLabel, type LauncherPresentationState } from "@shared/launcher-presentation";
import type { LauncherSettings } from "@shared/launcher-models";

export type SectionId = "status" | "environment" | "diagnostics" | "settings";
export type SectionTransitionState = "idle" | "exiting" | "entering";

export const serviceStateConfig: Record<LauncherPresentationState, { status: PresenceBadgeStatus; label: string }> = {
  stopped: { status: "offline", label: getLauncherStateLabel("stopped") },
  starting: { status: "busy", label: getLauncherStateLabel("starting") },
  running: { status: "available", label: getLauncherStateLabel("running") },
  degraded: { status: "busy", label: getLauncherStateLabel("degraded") },
  setup_required: { status: "blocked", label: getLauncherStateLabel("setup_required") },
  stopping: { status: "busy", label: getLauncherStateLabel("stopping") },
  failed: { status: "blocked", label: getLauncherStateLabel("failed") },
};

export const severityConfig = {
  error: { label: "阻塞", icon: <Warning20Filled /> },
  warning: { label: "警告", icon: <Warning20Filled /> },
  ok: { label: "正常", icon: <CheckmarkCircle20Filled /> },
};

export const sections = [
  { id: "status" as SectionId, title: "运行状态", icon: <Status20Filled /> },
  { id: "environment" as SectionId, title: "环境检查", icon: <HeartPulse20Filled /> },
  { id: "diagnostics" as SectionId, title: "日志诊断", icon: <DocumentText20Filled /> },
  { id: "settings" as SectionId, title: "偏好设置", icon: <Settings20Filled /> },
];

export const sectionContent = {
  status: {
    eyebrow: "Service Console",
    title: "运行状态",
    detail: "查看当前服务状态，并直接处理启动、停止、管理和恢复动作。",
  },
  environment: {
    eyebrow: "Environment Review",
    title: "环境检查",
    detail: "汇总启动前检查、恢复兼容性和本地访问信息。",
  },
  diagnostics: {
    eyebrow: "Diagnostics",
    title: "日志诊断",
    detail: "集中查看系统状态摘要与最近异常输出。",
  },
  settings: {
    eyebrow: "Launcher Settings",
    title: "偏好设置",
    detail: "管理安装路径、关闭行为和本地维护操作。",
  },
} satisfies Record<SectionId, { eyebrow: string; title: string; detail: string }>;

export const severityOrder = {
  error: 0,
  warning: 1,
  ok: 2,
} satisfies Record<"error" | "warning" | "ok", number>;

export const busyActionLabels: Record<string, string> = {
  initialize: "正在准备启动器",
  refresh: "正在刷新状态",
  start: "正在启动服务",
  stop: "正在停止服务",
  restart: "正在重启服务",
  save: "正在保存设置",
  "open-web": "正在打开管理面板",
  "open-release-page": "正在打开版本页面",
  "open-logs": "正在打开日志目录",
  "reset-admin": "正在重置本地凭据",
  "recovery-recheck": "正在复核恢复兼容性",
  "runtime-bootstrap": "正在准备运行环境",
  "open-plugin": "正在打开插件详情",
};

export const closeBehaviorOptions: Array<{
  value: LauncherSettings["closeBehavior"];
  label: string;
  detail: string;
}> = [
  { value: "ask_every_time", label: "每次询问", detail: "每次关闭窗口时都显示确认选项。" },
  { value: "hide_to_tray", label: "系统托盘", detail: "关闭主窗口后保留托盘入口和后台状态。" },
  { value: "exit_application", label: "完全退出", detail: "直接结束启动器窗口与托盘进程。" },
];

export function statusSummary(state: LauncherPresentationState): string {
  return getLauncherStateLabel(state);
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

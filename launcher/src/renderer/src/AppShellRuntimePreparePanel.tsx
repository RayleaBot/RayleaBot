import { ProgressBar, Text } from "@fluentui/react-components";
import type {
  RuntimePrepareResourceProgress,
  RuntimePrepareSnapshot,
} from "@shared/launcher-models";

type AppShellRuntimePreparePanelProps = {
  runtimePrepare: RuntimePrepareSnapshot | null;
};

const statusLabels: Record<string, string> = {
  pending: "待处理",
  running: "进行中",
  succeeded: "已完成",
  failed: "失败",
};

const stageLabels: Record<string, string> = {
  inspect: "检查",
  lock: "准备锁",
  download: "下载",
  verify: "校验",
  cleanup: "清理",
  extract: "解压",
  activate: "启用",
  complete: "完成",
  manifest: "清单",
  entrypoint: "入口文件",
};

function formatBytes(value: number | null) {
  if (!value || value <= 0) {
    return "";
  }
  const units = ["B", "KB", "MB", "GB"];
  let next = value;
  let unitIndex = 0;
  while (next >= 1024 && unitIndex < units.length - 1) {
    next /= 1024;
    unitIndex += 1;
  }
  return `${next.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function progressValue(item: RuntimePrepareResourceProgress) {
  if (item.progress === null) {
    return undefined;
  }
  return item.progress / 100;
}

function progressText(item: RuntimePrepareResourceProgress) {
  if (item.stage === "download") {
    const downloaded = formatBytes(item.downloadedBytes);
    const total = formatBytes(item.totalBytes);
    if (downloaded && total) {
      return `${downloaded} / ${total}`;
    }
    if (downloaded) {
      return `${downloaded}`;
    }
  }
  if (item.stage === "extract" && item.extractedEntries !== null && item.totalEntries !== null && item.totalEntries > 0) {
    return `${item.extractedEntries} / ${item.totalEntries} 个文件`;
  }
  if (item.progress !== null) {
    return `${item.progress}%`;
  }
  return "正在处理";
}

function resourceKey(item: RuntimePrepareResourceProgress) {
  return item.kind || item.resourceId || item.label;
}

function RuntimePrepareResourceItem({ item }: { item: RuntimePrepareResourceProgress }) {
  const value = progressValue(item);
  return (
    <div className={`runtime-prepare-item runtime-prepare-item--${item.status}`}>
      <div className="runtime-prepare-item__header">
        <div className="runtime-prepare-item__title">
          <Text weight="bold" size={200}>{item.label}</Text>
          <span className="runtime-prepare-item__stage">{stageLabels[item.stage] ?? item.stage}</span>
        </div>
        <span className={`runtime-prepare-status runtime-prepare-status--${item.status}`}>
          {statusLabels[item.status] ?? item.status}
        </span>
      </div>
      <div className="runtime-prepare-progress-row">
        <ProgressBar value={value} thickness="medium" />
        <span className="runtime-prepare-progress-text">{progressText(item)}</span>
      </div>
      <div className="runtime-prepare-details">
        {item.sourceUrl ? (
          <div><span>来源</span><code title={item.sourceUrl}>{item.sourceLabel ? `${item.sourceLabel} · ${item.sourceUrl}` : item.sourceUrl}</code></div>
        ) : null}
        {item.archivePath ? (
          <div><span>下载位置</span><code title={item.archivePath}>{item.archivePath}</code></div>
        ) : null}
        {item.storeRoot ? (
          <div><span>解压位置</span><code title={item.storeRoot}>{item.storeRoot}</code></div>
        ) : null}
        {item.error ? (
          <div><span>错误</span><code title={item.error}>{item.error}</code></div>
        ) : null}
      </div>
    </div>
  );
}

export function AppShellRuntimePreparePanel({ runtimePrepare }: AppShellRuntimePreparePanelProps) {
  if (!runtimePrepare || (!runtimePrepare.active && runtimePrepare.resources.length === 0)) {
    return null;
  }
  const current = runtimePrepare.resources.find((item) => item.kind === runtimePrepare.currentKind)
    ?? runtimePrepare.resources.at(-1)
    ?? null;

  return (
    <article className="panel glass-panel glass-panel--subtle runtime-prepare-panel">
      <div className="runtime-prepare-panel__header">
        <div>
          <div className="brand-eyebrow">运行环境准备</div>
          <Text size={200} className="panel-muted">
            {runtimePrepare.summary || current?.summary || "正在准备运行环境"}
          </Text>
        </div>
        {current ? (
          <span className={`runtime-prepare-status runtime-prepare-status--${current.status}`}>
            {statusLabels[current.status] ?? current.status}
          </span>
        ) : null}
      </div>
      <div className="runtime-prepare-list">
        {runtimePrepare.resources.map((item) => (
          <RuntimePrepareResourceItem key={resourceKey(item)} item={item} />
        ))}
      </div>
    </article>
  );
}

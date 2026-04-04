import { Button, Text } from "@fluentui/react-components";
import { FolderOpen20Filled } from "@fluentui/react-icons";

type AppShellStatusLogsProps = {
  hasRecentStderr: boolean;
  logAlert: "none" | "error";
  logHighlight: "none" | "fresh";
  logs: string[];
  onOpenLogs: () => void;
};

export function AppShellStatusLogs({
  hasRecentStderr,
  logAlert,
  logHighlight,
  logs,
  onOpenLogs,
}: AppShellStatusLogsProps) {
  return (
    <article className="status-log-panel panel glass-panel" data-alert={logAlert} data-highlight={logHighlight}>
      <div className="panel-header-row">
        <div className="brand-eyebrow">异常输出监控</div>
        <span className={`status-log-indicator status-log-indicator--${hasRecentStderr ? "alert" : "quiet"}`}>
          {hasRecentStderr ? "已检测到异常输出" : "当前无新异常"}
        </span>
      </div>
      {hasRecentStderr ? (
        <pre className="log-surface status-log-surface--modern">{logs.join("\n")}</pre>
      ) : (
        <div className="log-empty-state">
          <div className="log-empty-state__title">当前没有新的异常日志</div>
          <Text size={200} className="panel-muted">服务输出保持安静，完整日志仍可随时打开。</Text>
        </div>
      )}
      <div className="panel-footer-actions">
        <Button appearance="transparent" size="small" className="frost-button frost-button--ghost-bright" onClick={onOpenLogs} icon={<FolderOpen20Filled />}>查看完整日志</Button>
      </div>
    </article>
  );
}

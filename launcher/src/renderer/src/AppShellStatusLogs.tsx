import { Button } from "@fluentui/react-components";
import { CheckmarkCircle20Regular, FolderOpen20Regular } from "@fluentui/react-icons";

type AppShellStatusLogsProps = {
  hasRecentStderr: boolean;
  logs: string[];
  onOpenLogs: () => void;
};

export function AppShellStatusLogs({
  hasRecentStderr,
  logs,
  onOpenLogs,
}: AppShellStatusLogsProps) {
  const openLogs = (
    <Button appearance="secondary" className="launcher-button" data-emphasis="regular" onClick={onOpenLogs} icon={<FolderOpen20Regular />}>打开完整日志</Button>
  );

  if (!hasRecentStderr) {
    return (
      <section className="status-log-row content-group" data-alert="none" aria-labelledby="status-log-title">
        <div className="status-log-row__status" role="status">
          <span className="status-log-row__icon" aria-hidden="true">
            <CheckmarkCircle20Regular />
          </span>
          <div>
            <h3 id="status-log-title">异常输出</h3>
            <span>当前没有新的异常日志。</span>
          </div>
        </div>
        {openLogs}
      </section>
    );
  }

  return (
    <section className="status-log-panel content-group" data-alert="error" aria-labelledby="status-log-title">
      <div className="status-log-panel__heading">
        <h3 id="status-log-title">异常输出</h3>
        <span className="status-label" data-state="danger">已检测到异常输出</span>
      </div>
      <pre className="log-surface status-log-panel__surface">{logs.join("\n")}</pre>
      <div className="status-log-panel__footer">{openLogs}</div>
    </section>
  );
}

import { Button } from "@fluentui/react-components";
import { FolderOpen20Filled, CheckmarkCircle20Filled } from "@fluentui/react-icons";

type AppShellStatusLogsProps = {
  hasRecentStderr: boolean;
  logAlert: "none" | "error";
  logs: string[];
  onOpenLogs: () => void;
};

export function AppShellStatusLogs({
  hasRecentStderr,
  logAlert,
  logs,
  onOpenLogs,
}: AppShellStatusLogsProps) {
  if (!hasRecentStderr) {
    return (
      <section className="log-summary-row" data-alert={logAlert} aria-labelledby="status-log-title">
        <div className="log-summary-row__status" role="status">
          <span className="log-summary-row__icon" aria-hidden="true">
            <CheckmarkCircle20Filled />
          </span>
          <div>
            <h3 id="status-log-title">异常输出</h3>
            <span>当前没有新的异常日志。</span>
          </div>
        </div>
        <Button appearance="subtle" onClick={onOpenLogs} icon={<FolderOpen20Filled />}>打开完整日志</Button>
      </section>
    );
  }

  return (
    <section className="log-workspace" data-alert={logAlert} aria-labelledby="status-log-title">
      <div className="workspace-heading">
        <h3 id="status-log-title">异常输出</h3>
        <span className="status-label" data-state="danger">已检测到异常输出</span>
      </div>
      <pre className="log-surface status-log-surface">{logs.join("\n")}</pre>
      <div className="workspace-footer">
        <Button appearance="subtle" onClick={onOpenLogs} icon={<FolderOpen20Filled />}>打开完整日志</Button>
      </div>
    </section>
  );
}

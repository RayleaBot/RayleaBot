import { Button } from "@fluentui/react-components";
import { CheckmarkCircle20Regular, FolderOpen20Regular, Info20Regular } from "@fluentui/react-icons";

import { uncapturedOutputText } from "./AppShell.copy";

type AppShellStatusLogsProps = {
  externalService: boolean;
  logs: string[];
  onOpenLogs: () => void;
};

export function AppShellStatusLogs({
  externalService,
  logs,
  onOpenLogs,
}: AppShellStatusLogsProps) {
  const openLogs = (
    <Button appearance="secondary" className="launcher-button" data-emphasis="regular" onClick={onOpenLogs} icon={<FolderOpen20Regular />}>打开日志目录</Button>
  );

  if (logs.length === 0) {
    return (
      <section className="status-log-row content-group" data-alert={externalService ? "unknown" : "none"} aria-labelledby="status-log-title">
        <div className="status-log-row__status" role="status">
          <span className="status-log-row__icon" aria-hidden="true">
            {externalService ? <Info20Regular /> : <CheckmarkCircle20Regular />}
          </span>
          <div>
            <h3 id="status-log-title">异常输出</h3>
            <span>{externalService ? uncapturedOutputText : "当前没有新的异常输出。"}</span>
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

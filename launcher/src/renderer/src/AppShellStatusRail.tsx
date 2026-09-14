import { Button } from "@fluentui/react-components";

import { severityConfig } from "./AppShell.shared";

type RailCheck = {
  code: string;
  severity: string;
  title: string;
  summary: string;
};

type AppShellStatusRailProps = {
  canPrepareRuntime: boolean;
  checks: RailCheck[];
  onOpenTasks: () => void;
};

export function AppShellStatusRail({
  canPrepareRuntime,
  checks,
  onOpenTasks,
}: AppShellStatusRailProps) {
  const issueTone = checks.some((item) => item.severity === "error") ? "danger" : "warning";

  return (
    <aside className="status-attention-column" aria-label="需要关注的项目">
      {checks.length > 0 && (
        <section className="attention-panel" data-tone={issueTone}>
          <h3>环境问题</h3>
          <div className="attention-list">
          {checks.map((item) => (
            <div key={item.code} className="attention-list__item" data-severity={item.severity}>
              <span className="attention-list__icon">{severityConfig[item.severity as keyof typeof severityConfig]?.icon}</span>
              <div>
                <strong>{item.title}</strong>
                <p>
                  {item.code === "os.long_paths_unknown" && item.severity === "warning"
                    ? "无法确认长路径支持状态。若资源展开遇到限制，请手动检查系统长路径设置。"
                    : item.summary}
                </p>
              </div>
            </div>
          ))}
          </div>
        </section>
      )}

      {canPrepareRuntime ? (
        <section className="attention-panel" data-tone="attention">
          <h3>运行环境准备</h3>
          <p>检测到可由启动器准备的运行环境项。</p>
          <div className="button-row button-row--stackable">
            <Button appearance="subtle" onClick={onOpenTasks}>准备运行环境</Button>
          </div>
        </section>
      ) : null}
    </aside>
  );
}

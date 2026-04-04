import { Button, Text } from "@fluentui/react-components";

import { severityConfig } from "./AppShell.shared";

type RailCheck = {
  code: string;
  severity: string;
  title: string;
  summary: string;
};

type AppShellStatusRailProps = {
  canRecheckRecovery: boolean;
  canRunRecoveryActions: boolean;
  checks: RailCheck[];
  onOpenReleasePage: () => void;
  onRecoveryRecheck: () => void;
  onRuntimeBootstrap: () => void;
  recoveryStatusSummary: string;
  releaseSummary: string;
};

export function AppShellStatusRail({
  canRecheckRecovery,
  canRunRecoveryActions,
  checks,
  onOpenReleasePage,
  onRecoveryRecheck,
  onRuntimeBootstrap,
  recoveryStatusSummary,
  releaseSummary,
}: AppShellStatusRailProps) {
  return (
    <aside className="status-summary-rail status-side-column">
      {checks.length > 0 && (
        <div className="checks-stack checks-stack--side panel glass-panel glass-panel--subtle panel--side">
          <div className="brand-eyebrow brand-eyebrow--tight">环境预警</div>
          {checks.map((item) => (
            <div key={item.code} className={`check-item-mini check-item-mini--${item.severity}`}>
              <div className="check-item-mini__icon">{severityConfig[item.severity as keyof typeof severityConfig]?.icon}</div>
              <div className="check-item-mini__content">
                <Text weight="bold" size={200}>{item.title}</Text>
                <Text size={100} className="panel-muted">
                  {item.code === "os.long_paths_unknown" && item.severity === "warning"
                    ? "无法确认长路径支持状态。若资源展开遇到限制，请手动检查系统长路径设置。"
                    : item.summary}
                </Text>
                <Text size={100} className="panel-muted">{severityConfig[item.severity as keyof typeof severityConfig]?.label}</Text>
              </div>
            </div>
          ))}
        </div>
      )}

      <article className="panel glass-panel glass-panel--subtle panel--side">
        <div className="brand-eyebrow brand-eyebrow--tight">恢复兼容性</div>
        <Text size={200} className="panel-muted">{recoveryStatusSummary}</Text>
        <div className="side-actions-stack">
          <Button appearance="transparent" size="small" className="frost-button frost-button--secondary frost-button--block" onClick={onRecoveryRecheck} disabled={!canRecheckRecovery}>重新检查</Button>
          <Button appearance="transparent" size="small" className="frost-button frost-button--secondary frost-button--block" onClick={onRuntimeBootstrap} disabled={!canRunRecoveryActions}>准备运行环境</Button>
        </div>
      </article>

      <article className="panel glass-panel glass-panel--subtle panel--side">
        <div className="brand-eyebrow brand-eyebrow--tight">版本监控</div>
        <div className="version-status">
          <Text size={200} className="panel-muted">{releaseSummary}</Text>
        </div>
        <Button appearance="transparent" size="small" className="frost-button frost-button--secondary frost-button--block frost-button--outline" onClick={onOpenReleasePage}>检查更新</Button>
      </article>
    </aside>
  );
}

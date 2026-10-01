import { Button } from "@fluentui/react-components";
import type { LauncherDiagnosticIssue } from "@shared/launcher-models";

import { formatRuntimeResource } from "./AppShell.copy";
import { severityConfig } from "./AppShell.shared";

type RailCheck = {
  code: string;
  severity: string;
  title: string;
  summary: string;
};

type AppShellStatusRailProps = {
  checks: RailCheck[];
  runtimeResources: NonNullable<LauncherDiagnosticIssue["runtime_resources"]>;
  onOpenWeb: () => void;
};

export function AppShellStatusRail({
  checks,
  runtimeResources,
  onOpenWeb,
}: AppShellStatusRailProps) {
  const issueTone = checks.some((item) => item.severity === "error") ? "danger" : "warning";

  return (
    <aside className="status-attention-column" aria-label="需要关注的项目">
      {checks.length > 0 && (
        <section className="attention-panel content-group" data-tone={issueTone}>
          <h3>环境问题</h3>
          <div className="attention-list">
          {checks.map((item) => (
            <div key={item.code} className="attention-list__item" data-severity={item.severity}>
              <span className="attention-list__icon">{severityConfig[item.severity as keyof typeof severityConfig]?.icon}</span>
              <div>
                <strong>{item.title}</strong>
                <p>{item.summary}</p>
              </div>
            </div>
          ))}
          </div>
        </section>
      )}

      {runtimeResources.length > 0 ? (
        <section className="attention-panel content-group" data-tone="attention">
          <h3>运行环境准备</h3>
          <p>以下项目可在管理界面的系统状态页准备。</p>
          <ul className="attention-panel__items">
            {runtimeResources.map((resource) => <li key={resource}>{formatRuntimeResource(resource)}</li>)}
          </ul>
          <div className="button-row button-row--stackable">
            <Button appearance="secondary" className="launcher-button" data-emphasis="regular" onClick={onOpenWeb}>在管理界面准备</Button>
          </div>
        </section>
      ) : null}
    </aside>
  );
}

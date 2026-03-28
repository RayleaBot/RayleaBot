import { useMemo } from "react";
import { ChangeEvent, FormEvent } from "react";
import {
  Radio,
  RadioGroup,
  Input,
} from "@fluentui/react-components";
import type { InputOnChangeData } from "@fluentui/react-components";
import type {
  LauncherSettings,
  LauncherSnapshot,
  LauncherServiceState,
} from "@shared/launcher-models";

type SectionId = "status" | "environment" | "diagnostics" | "settings";

type AppShellProps = {
  snapshot: LauncherSnapshot;
  activeSection: SectionId;
  settingsDraft: LauncherSettings;
  editingSettings: boolean;
  diagnosticsSummary: string;
  busyAction: string | null;
  controlsDisabled: boolean;
  onNavigate: (section: SectionId) => void;
  onRefresh: () => void;
  onStart: () => void;
  onStop: () => void;
  onOpenWeb: () => void;
  onOpenReleasePage: () => void;
  onOpenLogs: () => void;
  onBeginEdit: () => void;
  onCancelEdit: () => void;
  onSaveSettings: () => void;
  onUpdateSettings: (partial: Partial<LauncherSettings>) => void;
  onChooseServer: () => void;
  onChooseConfig: () => void;
  onChooseWorkdir: () => void;
  onExit: () => void;
};

const sections = [
  { id: "status" as SectionId, title: "状态" },
  { id: "environment" as SectionId, title: "环境检查" },
  { id: "diagnostics" as SectionId, title: "日志与诊断" },
  { id: "settings" as SectionId, title: "设置" },
];

const severityRank: Record<string, number> = {
  error: 0,
  warning: 1,
  ok: 2,
};

function statusSummary(state: LauncherServiceState): string {
  switch (state) {
    case "stopped":
      return "未启动";
    case "starting":
      return "启动中";
    case "external_service":
    case "ready":
      return "运行中";
    case "degraded":
      return "受限运行";
    case "setup_required":
      return "需要设置";
    case "shutting_down":
      return "停止中";
    case "failed":
      return "启动失败";
    default:
      return "未知状态";
  }
}

export function AppShell({
  snapshot,
  activeSection,
  settingsDraft,
  editingSettings,
  diagnosticsSummary,
  busyAction,
  controlsDisabled,
  onNavigate,
  onRefresh,
  onStart,
  onStop,
  onOpenWeb,
  onOpenReleasePage,
  onOpenLogs,
  onBeginEdit,
  onCancelEdit,
  onSaveSettings,
  onUpdateSettings,
  onChooseServer,
  onChooseConfig,
  onChooseWorkdir,
  onExit,
}: AppShellProps) {
  const primaryIssue = useMemo(() => {
    return [...snapshot.environmentChecks].sort(
      (left, right) =>
        severityRank[left.severity] - severityRank[right.severity],
    )[0];
  }, [snapshot.environmentChecks]);

  const groupedChecks = useMemo(() => ({
    blocking: snapshot.environmentChecks.filter(
      (item) => item.severity === "error",
    ),
    warnings: snapshot.environmentChecks.filter(
      (item) => item.severity === "warning",
    ),
    ready: snapshot.environmentChecks.filter(
      (item) => item.severity === "ok",
    ),
  }), [snapshot.environmentChecks]);

  const trayStatus = useMemo(
    () => statusSummary(snapshot.serviceState),
    [snapshot.serviceState],
  );

  const startDisabled =
    controlsDisabled || busyAction === "start" || busyAction === "stop";
  const stopDisabled = controlsDisabled || busyAction === "stop";
  const genericDisabled = controlsDisabled;

  return (
    <div className="app-shell">
      <aside className="shell-sidebar">
        <div className="brand-card">
          <div className="brand-eyebrow">RayleaBot</div>
          <h1>RayleaBot 启动器</h1>
          <p>本地服务壳、环境检查和管理入口。</p>
        </div>

        <nav className="section-nav" aria-label="Primary">
          {sections.map((section) => (
            <button
              key={section.id}
              className={`nav-item${activeSection === section.id ? " active" : ""}`}
              type="button"
              onClick={() => onNavigate(section.id)}
            >
              {section.title}
            </button>
          ))}
        </nav>

        <div className="sidebar-summary">
          <span>当前状态</span>
          <strong>{trayStatus}</strong>
        </div>
        <div className="sidebar-summary subtle">
          <span>服务入口</span>
          <strong>{snapshot.endpoint.baseUrl}</strong>
        </div>
      </aside>

      <main className="shell-main">
        <section className="hero-card">
          <div className="hero-copy">
            <div className="hero-eyebrow">Service Control</div>
            <h2>{snapshot.serviceDetail}</h2>
            <p>
              {snapshot.lastError ||
                "查看当前状态、主操作和需要处理的问题。"}
            </p>
          </div>
          <div className="hero-actions">
            <button
              className="action primary"
              type="button"
              disabled={startDisabled}
              onClick={onStart}
            >
              {snapshot.serviceState === "external_service" ||
              snapshot.serviceState === "ready"
                ? "重新检查"
                : "启动服务"}
            </button>
            <button
              className="action"
              type="button"
              disabled={stopDisabled}
              onClick={onStop}
            >
              停止服务
            </button>
            <button
              className="action"
              type="button"
              disabled={genericDisabled}
              onClick={onOpenWeb}
            >
              打开管理界面
            </button>
            <button
              className="action ghost"
              type="button"
              disabled={genericDisabled}
              onClick={onRefresh}
            >
              刷新状态
            </button>
          </div>
        </section>

        {primaryIssue && (
          <section
            className={`issue-banner ${primaryIssue.severity}`}
          >
            <div>
              <strong>{primaryIssue.title}</strong>
              <p>{primaryIssue.summary}</p>
            </div>
            <p>{primaryIssue.remediation || primaryIssue.detail}</p>
          </section>
        )}

        {activeSection === "status" && (
          <section className="content-grid">
            <article className="panel panel-primary">
              <h3>服务信息</h3>
              <dl className="kv-grid">
                <div>
                  <dt>状态</dt>
                  <dd>{trayStatus}</dd>
                </div>
                <div>
                  <dt>服务入口</dt>
                  <dd>{snapshot.endpoint.baseUrl}</dd>
                </div>
                <div>
                  <dt>工作目录</dt>
                  <dd>{snapshot.settings.workdir}</dd>
                </div>
                <div>
                  <dt>PID</dt>
                  <dd>{snapshot.processId ?? "—"}</dd>
                </div>
              </dl>
            </article>

            <article className="panel">
              <h3>版本与发布</h3>
              <p>{snapshot.releaseCheck.summary}</p>
              <button
                className="action ghost"
                type="button"
                onClick={onOpenReleasePage}
              >
                打开发布页
              </button>
            </article>

            <article className="panel full">
              <h3>最近错误输出</h3>
              <pre className="log-surface">
                {snapshot.recentStderr.join("\n") ||
                  "当前没有新的错误。"}
              </pre>
              <button
                className="action ghost"
                type="button"
                onClick={onOpenLogs}
              >
                打开日志目录
              </button>
            </article>
          </section>
        )}

        {activeSection === "environment" && (
          <section className="content-grid">
            <article className="panel metric-panel">
              <div className="metric">
                <span>阻塞项</span>
                <strong>{groupedChecks.blocking.length}</strong>
              </div>
              <div className="metric">
                <span>需注意</span>
                <strong>{groupedChecks.warnings.length}</strong>
              </div>
              <div className="metric">
                <span>正常项</span>
                <strong>{groupedChecks.ready.length}</strong>
              </div>
            </article>

            <article className="panel full">
              <h3>环境检查结果</h3>
              <ul className="check-list">
                {snapshot.environmentChecks.map((item) => (
                  <li key={item.code} className={item.severity}>
                    <div>
                      <strong>{item.title}</strong>
                      <p>{item.summary}</p>
                    </div>
                    <p>
                      {item.detail}{" "}
                      {item.remediation}
                    </p>
                  </li>
                ))}
              </ul>
            </article>
          </section>
        )}

        {activeSection === "diagnostics" && (
          <section className="content-grid">
            <article className="panel full">
              <h3>诊断摘要</h3>
              <pre className="log-surface">{diagnosticsSummary}</pre>
            </article>
          </section>
        )}

        {activeSection === "settings" && (
          <section className="content-grid">
            <article className="panel full">
              <div className="settings-header">
                <div>
                  <h3>本地设置</h3>
                  <p>管理本地路径和关闭策略。</p>
                </div>
                <div className="settings-actions">
                  {!editingSettings ? (
                    <button
                      className="action ghost"
                      type="button"
                      disabled={genericDisabled}
                      onClick={onBeginEdit}
                    >
                      编辑路径
                    </button>
                  ) : (
                    <>
                      <button
                        className="action ghost"
                        type="button"
                        disabled={genericDisabled}
                        onClick={onCancelEdit}
                      >
                        取消编辑
                      </button>
                      <button
                        className="action primary"
                        type="button"
                        disabled={genericDisabled}
                        onClick={onSaveSettings}
                      >
                        保存设置
                      </button>
                    </>
                  )}
                </div>
              </div>

              <div className="settings-grid">
                <label className="field">
                  <span>服务端可执行文件</span>
                  <div className="field-inline">
                    <Input
                      value={settingsDraft.serverExecutablePath}
                      readOnly={!editingSettings}
                      onChange={(_: ChangeEvent<HTMLInputElement>, data: InputOnChangeData) =>
                        onUpdateSettings({
                          serverExecutablePath: data.value,
                        })
                      }
                      className="field-input"
                    />
                    <button
                      className="action ghost"
                      type="button"
                      disabled={!editingSettings}
                      onClick={onChooseServer}
                    >
                      浏览
                    </button>
                  </div>
                </label>

                <label className="field">
                  <span>用户配置文件</span>
                  <div className="field-inline">
                    <Input
                      value={settingsDraft.configPath}
                      readOnly={!editingSettings}
                      onChange={(_: ChangeEvent<HTMLInputElement>, data: InputOnChangeData) =>
                        onUpdateSettings({ configPath: data.value })
                      }
                      className="field-input"
                    />
                    <button
                      className="action ghost"
                      type="button"
                      disabled={!editingSettings}
                      onClick={onChooseConfig}
                    >
                      浏览
                    </button>
                  </div>
                </label>

                <label className="field">
                  <span>工作目录</span>
                  <div className="field-inline">
                    <Input
                      value={settingsDraft.workdir}
                      readOnly={!editingSettings}
                      onChange={(_: ChangeEvent<HTMLInputElement>, data: InputOnChangeData) =>
                        onUpdateSettings({ workdir: data.value })
                      }
                      className="field-input"
                    />
                    <button
                      className="action ghost"
                      type="button"
                      disabled={!editingSettings}
                      onClick={onChooseWorkdir}
                    >
                      选择目录
                    </button>
                  </div>
                </label>
              </div>

              <RadioGroup
                value={settingsDraft.closeBehavior}
                onChange={(_: FormEvent<HTMLElement>, data: { value: string }) =>
                  onUpdateSettings({
                    closeBehavior: data.value as LauncherSettings["closeBehavior"],
                  })
                }
              >
                <div className="close-behavior">
                  <label className="radio-label">
                    <Radio
                      value="ask_every_time"
                      disabled={!editingSettings}
                    />
                    每次询问
                  </label>
                  <label className="radio-label">
                    <Radio value="hide_to_tray" disabled={!editingSettings} />
                    隐藏到托盘
                  </label>
                  <label className="radio-label">
                    <Radio
                      value="exit_application"
                      disabled={!editingSettings}
                    />
                    完全退出
                  </label>
                </div>
              </RadioGroup>

              <button className="action danger" type="button" onClick={onExit}>
                完全退出启动器
              </button>
            </article>
          </section>
        )}
      </main>
    </div>
  );
}

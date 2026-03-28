import { useMemo } from "react";
import { ChangeEvent, FormEvent } from "react";
import {
  Radio,
  RadioGroup,
  Input,
  PresenceBadge,
  Text,
  Button,
  Badge,
  Field,
} from "@fluentui/react-components";
import type { PresenceBadgeStatus } from "@fluentui/react-components";
import { MessageBar } from "@fluentui/react-message-bar";
import {
  Play24Filled,
  Stop24Filled,
  Globe24Filled,
  FolderOpen24Filled,
  CheckmarkCircle24Filled,
  Warning24Filled,
  DismissCircle24Filled,
} from "@fluentui/react-icons";
import type { InputOnChangeData } from "@fluentui/react-components";
import type {
  LauncherSettings,
  LauncherSnapshot,
  LauncherServiceState,
} from "@shared/launcher-models";

type SectionId = "status" | "environment" | "diagnostics" | "settings";

const serviceStateConfig: Record<
  LauncherServiceState,
  { status: PresenceBadgeStatus; label: string }
> = {
  stopped: { status: "offline", label: "已停止" },
  starting: { status: "busy", label: "启动中" },
  external_service: { status: "available", label: "运行中" },
  ready: { status: "available", label: "运行中" },
  degraded: { status: "busy", label: "受限运行" },
  setup_required: { status: "blocked", label: "需要设置" },
  shutting_down: { status: "busy", label: "停止中" },
  failed: { status: "blocked", label: "启动失败" },
};

const severityConfig = {
  error: { color: "danger" as const, label: "阻塞", icon: <DismissCircle24Filled /> },
  warning: { color: "warning" as const, label: "警告", icon: <Warning24Filled /> },
  ok: { color: "success" as const, label: "正常", icon: <CheckmarkCircle24Filled /> },
};

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
        <Button
          appearance="secondary"
          size="small"
          disabled={genericDisabled}
          onClick={onRefresh}
          style={{ marginTop: "8px" }}
        >
          刷新状态
        </Button>
      </aside>

      <main className="shell-main">
        <section className="hero-card">
          <div className="hero-copy">
            <div className="hero-eyebrow">Service Control</div>
            <div style={{ display: "flex", alignItems: "center", gap: "8px", marginBottom: "8px" }}>
              <PresenceBadge
                status={serviceStateConfig[snapshot.serviceState]?.status ?? "unknow"}
                size="large"
              />
              <Text weight="semibold" size={500}>
                {serviceStateConfig[snapshot.serviceState]?.label ?? "未知状态"}
              </Text>
            </div>
            <p style={{ color: "rgba(220, 235, 255, 0.78)" }}>{snapshot.serviceDetail}</p>
            {snapshot.lastError && (
              <MessageBar intent="error" className="message-bar" style={{ marginTop: "12px" }}>
                {snapshot.lastError}
              </MessageBar>
            )}
          </div>
          <div className="hero-actions">
            <Button
              appearance="primary"
              size="large"
              disabled={startDisabled}
              onClick={onStart}
              icon={<Play24Filled />}
            >
              {snapshot.serviceState === "external_service" ||
              snapshot.serviceState === "ready"
                ? "重新检查"
                : "启动服务"}
            </Button>
            <Button
              appearance="secondary"
              size="large"
              disabled={stopDisabled}
              onClick={onStop}
              icon={<Stop24Filled />}
            >
              停止服务
            </Button>
            <Button
              appearance="outline"
              size="large"
              disabled={genericDisabled}
              onClick={onOpenWeb}
              icon={<Globe24Filled />}
            >
              打开管理界面
            </Button>
          </div>
        </section>

        {primaryIssue && (
          <MessageBar
            intent={primaryIssue.severity === "error" ? "error" : "warning"}
            icon={primaryIssue.severity === "error" ? <DismissCircle24Filled /> : <Warning24Filled />}
            className="message-bar"
          >
            <Text weight="semibold">{primaryIssue.title}</Text>
            <Text>{primaryIssue.summary}</Text>
            <Text size={200}>{primaryIssue.remediation || primaryIssue.detail}</Text>
          </MessageBar>
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
              <Button
                appearance="secondary"
                onClick={onOpenReleasePage}
              >
                打开发布页
              </Button>
            </article>

            <article className="panel full">
              <h3>最近错误输出</h3>
              <pre className="log-surface">
                {snapshot.recentStderr.join("\n") ||
                  "当前没有新的错误。"}
              </pre>
              <Button
                appearance="secondary"
                onClick={onOpenLogs}
              >
                打开日志目录
              </Button>
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
              <div style={{ display: "grid", gap: "12px" }}>
                {snapshot.environmentChecks.map((item) => {
                  const c = severityConfig[item.severity] ?? severityConfig.ok;
                  return (
                    <MessageBar
                      key={item.code}
                      intent={item.severity === "ok" ? "success" : item.severity === "warning" ? "warning" : "error"}
                      icon={c.icon}
                      className="message-bar"
                    >
                      <div style={{ display: "flex", alignItems: "center", gap: "8px", marginBottom: "4px" }}>
                        <Text weight="semibold">{item.title}</Text>
                        <Badge appearance="filled" color={c.color}>{c.label}</Badge>
                      </div>
                      <Text>{item.summary}</Text>
                      {item.remediation && (
                        <Text size={200} style={{ color: "var(--colorNeutralForeground3)" }}>
                          解决方法: {item.remediation}
                        </Text>
                      )}
                    </MessageBar>
                  );
                })}
              </div>
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
              <div
                className="settings-header"
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  marginBottom: "18px",
                  padding: editingSettings ? "12px 16px" : "0",
                  background: editingSettings ? "rgba(13, 32, 48, 0.5)" : "transparent",
                  borderRadius: "8px",
                  transition: "all 200ms ease",
                }}
              >
                <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                  <Text weight="semibold" size={500}>本地设置</Text>
                  {editingSettings && (
                    <Badge appearance="tint" color="brand">编辑模式</Badge>
                  )}
                </div>
                <div className="settings-actions">
                  {!editingSettings ? (
                    <Button
                      appearance="secondary"
                      disabled={genericDisabled}
                      onClick={onBeginEdit}
                    >
                      编辑路径
                    </Button>
                  ) : (
                    <>
                      <Button
                        appearance="secondary"
                        disabled={genericDisabled}
                        onClick={onCancelEdit}
                      >
                        取消编辑
                      </Button>
                      <Button
                        appearance="primary"
                        disabled={genericDisabled}
                        onClick={onSaveSettings}
                      >
                        保存设置
                      </Button>
                    </>
                  )}
                </div>
              </div>

              <div className="settings-grid">
                <Field label="服务端可执行文件">
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
                    <Button
                      appearance="secondary"
                      disabled={!editingSettings}
                      onClick={onChooseServer}
                      icon={<FolderOpen24Filled />}
                    >
                      浏览
                    </Button>
                  </div>
                </Field>

                <Field label="用户配置文件">
                  <div className="field-inline">
                    <Input
                      value={settingsDraft.configPath}
                      readOnly={!editingSettings}
                      onChange={(_: ChangeEvent<HTMLInputElement>, data: InputOnChangeData) =>
                        onUpdateSettings({ configPath: data.value })
                      }
                      className="field-input"
                    />
                    <Button
                      appearance="secondary"
                      disabled={!editingSettings}
                      onClick={onChooseConfig}
                      icon={<FolderOpen24Filled />}
                    >
                      浏览
                    </Button>
                  </div>
                </Field>

                <Field label="工作目录">
                  <div className="field-inline">
                    <Input
                      value={settingsDraft.workdir}
                      readOnly={!editingSettings}
                      onChange={(_: ChangeEvent<HTMLInputElement>, data: InputOnChangeData) =>
                        onUpdateSettings({ workdir: data.value })
                      }
                      className="field-input"
                    />
                    <Button
                      appearance="secondary"
                      disabled={!editingSettings}
                      onClick={onChooseWorkdir}
                      icon={<FolderOpen24Filled />}
                    >
                      选择目录
                    </Button>
                  </div>
                </Field>
              </div>

              <Field label="关闭行为">
                <RadioGroup
                  value={settingsDraft.closeBehavior}
                  onChange={(_: FormEvent<HTMLElement>, data: { value: string }) => {
                    if (editingSettings) {
                      onUpdateSettings({
                        closeBehavior: data.value as LauncherSettings["closeBehavior"],
                      });
                    }
                  }}
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
              </Field>

              <Button
                appearance="primary"
                onClick={onExit}
                style={{ background: "linear-gradient(135deg, #b44d63, #6e2433)", border: "none" }}
              >
                完全退出启动器
              </Button>
            </article>
          </section>
        )}
      </main>
    </div>
  );
}

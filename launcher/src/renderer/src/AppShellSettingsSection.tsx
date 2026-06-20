import { Button, Input, Radio, RadioGroup, Text } from "@fluentui/react-components";
import {
  DocumentText20Filled,
  FolderOpen20Filled,
  Status20Filled,
  Stop20Filled,
  Warning20Filled,
} from "@fluentui/react-icons";
import { useEffect, useState } from "react";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type {
  LauncherAdvancedOverrides,
  LauncherResolvedSettings,
  LauncherSettings,
  LauncherSnapshot,
} from "@shared/launcher-models";

import { closeBehaviorOptions } from "./AppShell.shared";

type SettingsSectionProps = {
  snapshot: LauncherSnapshot;
  settingsDraft: LauncherSettings;
  resolvedSettings: LauncherResolvedSettings;
  editingSettings: boolean;
  busyAction: string | null;
  controlsDisabled: boolean;
  onUpdateInstallationRoot: (value: string) => void;
  onUpdateCloseBehavior: (value: LauncherSettings["closeBehavior"]) => void;
  onUpdateAdvancedOverride: (key: keyof LauncherAdvancedOverrides, value: string) => void;
  onChooseInstallationRoot: () => void;
  onChooseServer: () => void;
  onChooseConfig: () => void;
  onChooseWorkdir: () => void;
  onResetAdmin: () => void;
  onExit: () => void;
};

export function AppShellSettingsSection({
  snapshot,
  settingsDraft,
  resolvedSettings,
  editingSettings,
  busyAction,
  controlsDisabled,
  onUpdateInstallationRoot,
  onUpdateCloseBehavior,
  onUpdateAdvancedOverride,
  onChooseInstallationRoot,
  onChooseServer,
  onChooseConfig,
  onChooseWorkdir,
  onResetAdmin,
  onExit,
}: SettingsSectionProps) {
  const [showAdvancedOverrides, setShowAdvancedOverrides] = useState(false);
  const presentation = deriveLauncherPresentation(snapshot);

  const hasAdvancedOverrides = Boolean(
    settingsDraft.advancedOverrides?.serverExecutablePath
      || settingsDraft.advancedOverrides?.configPath
      || settingsDraft.advancedOverrides?.workdir,
  );

  useEffect(() => {
    if (hasAdvancedOverrides) {
      setShowAdvancedOverrides(true);
    }
  }, [hasAdvancedOverrides]);

  const settingsSurfaceTag = editingSettings ? "当前草稿" : "当前值";

  return (
    <article className="panel glass-panel settings-panel" data-busy={busyAction ?? "idle"}>
      {editingSettings && (
        <div className="settings-edit-bar glass-panel glass-panel--subtle">
          <div className="settings-edit-status">
            <span className="settings-edit-status__dot" aria-hidden="true"></span>
            <div className="settings-edit-status__copy">
              <div className="settings-edit-status__title">编辑中</div>
              <Text size={200} className="settings-edit-status__detail">当前显示草稿路径与预览结果，保存后才会切换为生效值。</Text>
            </div>
          </div>
        </div>
      )}

      <div className="settings-compare-strip">
        <div className="settings-compare-card">
          <span className="settings-surface-tag">{settingsSurfaceTag}</span>
          <span className="settings-compare-card__label">安装目录</span>
          <span className="settings-compare-card__value" title={settingsDraft.installationRoot}>{settingsDraft.installationRoot || "—"}</span>
        </div>
        <div className="settings-compare-card settings-compare-card--resolved">
          <span className="settings-surface-tag settings-surface-tag--resolved">当前生效</span>
          <span className="settings-compare-card__label">进程工作目录</span>
          <span className="settings-compare-card__value" title={resolvedSettings.workdir}>{resolvedSettings.workdir || "—"}</span>
        </div>
      </div>

      <div className="settings-layout">
        <div className="settings-column settings-column--primary">
          <section className="settings-section glass-panel glass-panel--subtle">
            <div className="settings-section__header">
              <FolderOpen20Filled className="settings-section__icon" />
              <div className="panel-copy">
                <div className="brand-eyebrow brand-eyebrow--tight">安装目录</div>
                <Text size={200} className="panel-muted">启动器和工作服务的根目录位置</Text>
              </div>
              <span className="settings-surface-tag">{settingsSurfaceTag}</span>
            </div>
            <div className="path-field">
              <div className="path-control">
                <Input aria-label="安装目录" value={settingsDraft.installationRoot} readOnly={!editingSettings} className="frost-input frost-input--path" onChange={(_, data) => onUpdateInstallationRoot(data.value)} />
                <Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseInstallationRoot} icon={<FolderOpen20Filled />}>浏览</Button>
              </div>
            </div>
          </section>

          <section className={`settings-section glass-panel glass-panel--subtle ${showAdvancedOverrides ? "is-expanded" : ""}`}>
            <button type="button" className="settings-section__toggle" aria-expanded={showAdvancedOverrides} aria-label={showAdvancedOverrides ? "收起高级覆盖" : "展开高级覆盖"} onClick={() => setShowAdvancedOverrides((current) => !current)}>
              <div className="settings-section__header">
                <DocumentText20Filled className="settings-section__icon" />
                <div className="panel-copy">
                  <div className="brand-eyebrow brand-eyebrow--tight">高级覆盖</div>
                  <Text size={200} className="panel-muted">使用显式路径覆盖自动推导结果</Text>
                </div>
                <span className="settings-surface-tag">{settingsSurfaceTag}</span>
              </div>
              <span className="settings-section__chevron" aria-hidden="true">{showAdvancedOverrides ? "收起高级覆盖" : "展开高级覆盖"}</span>
            </button>

            {showAdvancedOverrides && (
              <div className="settings-advanced-fields">
                <label className="path-field"><span className="path-field__label">服务端覆盖</span><div className="path-control"><Input aria-label="服务端覆盖" value={settingsDraft.advancedOverrides?.serverExecutablePath ?? ""} readOnly={!editingSettings} placeholder={resolvedSettings.serverExecutablePath} className="frost-input frost-input--path" onChange={(_, data) => onUpdateAdvancedOverride("serverExecutablePath", data.value)} /><Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseServer} icon={<FolderOpen20Filled />}>浏览</Button></div></label>
                <label className="path-field"><span className="path-field__label">配置覆盖</span><div className="path-control"><Input aria-label="配置覆盖" value={settingsDraft.advancedOverrides?.configPath ?? ""} readOnly={!editingSettings} placeholder={resolvedSettings.configPath} className="frost-input frost-input--path" onChange={(_, data) => onUpdateAdvancedOverride("configPath", data.value)} /><Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseConfig} icon={<FolderOpen20Filled />}>浏览</Button></div></label>
                <label className="path-field"><span className="path-field__label">进程工作目录覆盖</span><div className="path-control"><Input aria-label="进程工作目录覆盖" value={settingsDraft.advancedOverrides?.workdir ?? ""} readOnly={!editingSettings} placeholder={resolvedSettings.workdir} className="frost-input frost-input--path" onChange={(_, data) => onUpdateAdvancedOverride("workdir", data.value)} /><Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseWorkdir} icon={<FolderOpen20Filled />}>选择</Button></div></label>
              </div>
            )}

            <div className="settings-resolution-panel">
              <div className="settings-resolution-panel__header">
                <Status20Filled className="settings-resolution-panel__icon" />
                <div className="panel-copy">
                  <div className="brand-eyebrow brand-eyebrow--tight">当前解析结果</div>
                  <Text size={200} className="panel-muted">当前生效的服务端、配置与进程工作目录路径。</Text>
                </div>
                <span className="settings-surface-tag settings-surface-tag--resolved">当前生效</span>
              </div>
              <div className="settings-info-list">
                <div className="settings-info-item">
                  <span className="settings-info-item__label">服务端</span>
                  <span className="settings-info-item__value" title={resolvedSettings.serverExecutablePath}>{resolvedSettings.serverExecutablePath}</span>
                </div>
                <div className="settings-info-item">
                  <span className="settings-info-item__label">配置</span>
                  <span className="settings-info-item__value" title={resolvedSettings.configPath}>{resolvedSettings.configPath}</span>
                </div>
                <div className="settings-info-item">
                  <span className="settings-info-item__label">进程工作目录</span>
                  <span className="settings-info-item__value" title={resolvedSettings.workdir}>{resolvedSettings.workdir}</span>
                </div>
              </div>
            </div>
          </section>
        </div>

        <div className="settings-column settings-column--secondary">
          <section className="preferences-panel glass-panel glass-panel--subtle">
            <div className="panel-copy">
              <div className="brand-eyebrow brand-eyebrow--tight">退出行为偏好</div>
              <Text size={200} className="panel-muted">关闭窗口时采用的默认动作。托盘模式会保留后台入口。</Text>
            </div>
            <RadioGroup value={settingsDraft.closeBehavior} onChange={(_, data) => onUpdateCloseBehavior(data.value as LauncherSettings["closeBehavior"])}>
              <div className="preference-options">
                {closeBehaviorOptions.map((option) => (
                  <label key={option.value} className={`preference-option${settingsDraft.closeBehavior === option.value ? " is-selected" : ""}${!editingSettings ? " is-disabled" : ""}`}>
                    <Radio className="preference-radio" value={option.value} disabled={!editingSettings} />
                    <span className="preference-option__body">
                      <span className="preference-option__title">{option.label}</span>
                      <span className="preference-option__detail">{option.detail}</span>
                    </span>
                  </label>
                ))}
              </div>
            </RadioGroup>
          </section>

          <section className="maintenance-panel glass-panel glass-panel--subtle">
            <div className="panel-copy">
              <div className="brand-eyebrow brand-eyebrow--tight">维护操作</div>
              <Text size={200} className="panel-muted">用于重置本地凭据或直接结束启动器进程。</Text>
            </div>
            <div className="maintenance-action-list">
              <div className="maintenance-action-card maintenance-action-card--danger">
                <div className="maintenance-action-card__lead">
                  <span className="maintenance-action-card__badge" aria-hidden="true"><Warning20Filled /></span>
                  <div className="maintenance-action-card__copy">
                    <div className="maintenance-action-card__title">重置凭据</div>
                    <Text size={200} className="maintenance-action-card__detail">清除本地管理凭据，下次启动时重新完成初始化。</Text>
                  </div>
                </div>
                <Button appearance="transparent" size="small" className="frost-button frost-button--danger maintenance-action-card__button" onClick={onResetAdmin} disabled={controlsDisabled || presentation.state === "starting" || presentation.state === "stopping"}>立即重置</Button>
              </div>
              <div className="maintenance-action-card maintenance-action-card--soft">
                <div className="maintenance-action-card__lead">
                  <span className="maintenance-action-card__badge" aria-hidden="true"><Stop20Filled /></span>
                  <div className="maintenance-action-card__copy">
                    <div className="maintenance-action-card__title">退出启动器</div>
                    <Text size={200} className="maintenance-action-card__detail">关闭窗口和托盘入口，不影响已保存配置与服务文件。</Text>
                  </div>
                </div>
                <Button appearance="transparent" size="small" className="frost-button frost-button--secondary maintenance-action-card__button" onClick={onExit}>退出启动器</Button>
              </div>
            </div>
          </section>
        </div>
      </div>
    </article>
  );
}

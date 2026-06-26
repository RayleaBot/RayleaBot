import type {
  LauncherAdvancedOverrides,
  LauncherResolvedSettings,
  LauncherSettings,
  LauncherSnapshot,
} from "@shared/launcher-models";

import type { SectionId, SectionTransitionState } from "./AppShell.shared";
import { AppShellAboutSection } from "./AppShellAboutSection";
import { AppShellChrome } from "./AppShellChrome";
import { AppShellDiagnosticsSection } from "./AppShellDiagnosticsSection";
import { AppShellEnvironmentSection } from "./AppShellEnvironmentSection";
import { AppShellSectionHeader } from "./AppShellSectionHeader";
import { AppShellSettingsSection } from "./AppShellSettingsSection";
import { AppShellStatusSection } from "./AppShellStatusSection";

export type AppShellViewProps = {
  snapshot: LauncherSnapshot;
  activeSection: SectionId;
  renderedSection: SectionId;
  sectionTransitionState: SectionTransitionState;
  platformLabel?: string;
  settingsDraft: LauncherSettings;
  resolvedSettings: LauncherResolvedSettings;
  editingSettings: boolean;
  diagnosticsSummary: string;
  busyAction: string | null;
  controlsDisabled: boolean;
  isMaximized: boolean;
  onNavigate: (section: SectionId) => void;
  onRefresh: () => void;
  onStart: () => void;
  onStop: () => void;
  onOpenWeb: () => void;
  onOpenRecoveryTasks: () => void;
  onOpenRuntimeTasks: () => void;
  onOpenRecoveryPlugin: (pluginId: string) => void;
  onCheckForUpdates: () => void;
  onDownloadUpdate: () => void;
  onInstallDownloadedUpdate: () => void;
  onOpenRepositoryPage: () => void;
  onOpenLogs: () => void;
  onResetAdmin: () => void;
  onBeginEdit: () => void;
  onCancelEdit: () => void;
  onSaveSettings: () => void;
  onUpdateInstallationRoot: (value: string) => void;
  onUpdateCloseBehavior: (value: LauncherSettings["closeBehavior"]) => void;
  onUpdateAdvancedOverride: (key: keyof LauncherAdvancedOverrides, value: string) => void;
  onChooseInstallationRoot: () => void;
  onChooseServer: () => void;
  onChooseConfig: () => void;
  onChooseWorkdir: () => void;
  onExit: () => void;
};

export function AppShellView({
  snapshot,
  activeSection,
  renderedSection,
  sectionTransitionState,
  platformLabel = "",
  settingsDraft,
  resolvedSettings,
  editingSettings,
  diagnosticsSummary,
  busyAction,
  controlsDisabled,
  isMaximized,
  onNavigate,
  onRefresh,
  onStart,
  onStop,
  onOpenWeb,
  onOpenRecoveryTasks,
  onOpenRuntimeTasks,
  onCheckForUpdates,
  onDownloadUpdate,
  onInstallDownloadedUpdate,
  onOpenRepositoryPage,
  onOpenLogs,
  onResetAdmin,
  onBeginEdit,
  onCancelEdit,
  onSaveSettings,
  onUpdateInstallationRoot,
  onUpdateCloseBehavior,
  onUpdateAdvancedOverride,
  onChooseInstallationRoot,
  onChooseServer,
  onChooseConfig,
  onChooseWorkdir,
  onExit,
}: AppShellViewProps) {
  return (
    <div className="app-shell">
      <AppShellChrome
        snapshot={snapshot}
        activeSection={activeSection}
        isMaximized={isMaximized}
        onNavigate={onNavigate}
        onRefresh={onRefresh}
      />

      <main className={`shell-main ${renderedSection === "environment" ? "active-environment" : ""}`} data-active-section={activeSection} data-rendered-section={renderedSection} data-transition={sectionTransitionState}>
        <div className="section-shell" data-section={renderedSection} data-transition={sectionTransitionState}>
          <AppShellSectionHeader
            snapshot={snapshot}
            renderedSection={renderedSection}
            busyAction={busyAction}
            controlsDisabled={controlsDisabled}
            editingSettings={editingSettings}
            onRefresh={onRefresh}
            onOpenRuntimeTasks={onOpenRuntimeTasks}
            onOpenLogs={onOpenLogs}
            onBeginEdit={onBeginEdit}
            onCancelEdit={onCancelEdit}
            onSaveSettings={onSaveSettings}
          />

          <div className="section-shell__content">
            {renderedSection === "status" && (
              <AppShellStatusSection
                snapshot={snapshot}
                resolvedSettings={resolvedSettings}
                busyAction={busyAction}
                controlsDisabled={controlsDisabled}
                onStart={onStart}
                onStop={onStop}
                onOpenWeb={onOpenWeb}
                onOpenRecoveryTasks={onOpenRecoveryTasks}
                onOpenRuntimeTasks={onOpenRuntimeTasks}
                onOpenLogs={onOpenLogs}
              />
            )}

            {renderedSection === "environment" && (
              <AppShellEnvironmentSection
                snapshot={snapshot}
                platformLabel={platformLabel}
              />
            )}

            {renderedSection === "diagnostics" && (
              <AppShellDiagnosticsSection
                snapshot={snapshot}
                diagnosticsSummary={diagnosticsSummary}
              />
            )}

            {renderedSection === "settings" && (
              <AppShellSettingsSection
                snapshot={snapshot}
                settingsDraft={settingsDraft}
                resolvedSettings={resolvedSettings}
                editingSettings={editingSettings}
                busyAction={busyAction}
                controlsDisabled={controlsDisabled}
                onUpdateInstallationRoot={onUpdateInstallationRoot}
                onUpdateCloseBehavior={onUpdateCloseBehavior}
                onUpdateAdvancedOverride={onUpdateAdvancedOverride}
                onChooseInstallationRoot={onChooseInstallationRoot}
                onChooseServer={onChooseServer}
                onChooseConfig={onChooseConfig}
                onChooseWorkdir={onChooseWorkdir}
                onResetAdmin={onResetAdmin}
                onExit={onExit}
              />
            )}

            {renderedSection === "about" && (
              <AppShellAboutSection
                snapshot={snapshot}
                controlsDisabled={controlsDisabled}
                onCheckForUpdates={onCheckForUpdates}
                onDownloadUpdate={onDownloadUpdate}
                onInstallDownloadedUpdate={onInstallDownloadedUpdate}
                onOpenRepositoryPage={onOpenRepositoryPage}
              />
            )}
          </div>
        </div>
      </main>
    </div>
  );
}

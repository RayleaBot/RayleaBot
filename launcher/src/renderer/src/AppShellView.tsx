import type {
  LauncherAdvancedOverrides,
  LauncherResolvedSettings,
  LauncherSettings,
  LauncherSnapshot,
} from "@shared/launcher-models";

import type { SectionId } from "./AppShell.shared";
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
  onOpenTasks: () => void;
  onCheckForUpdates: () => void;
  onDownloadUpdate: () => void;
  onInstallDownloadedUpdate: () => void;
  onOpenReleasePage: () => void;
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
  onOpenTasks,
  onCheckForUpdates,
  onDownloadUpdate,
  onInstallDownloadedUpdate,
  onOpenReleasePage,
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

      <main className={`shell-main active-${activeSection}`} data-active-section={activeSection}>
        <div className="section-shell" data-section={activeSection}>
          <AppShellSectionHeader
            snapshot={snapshot}
            renderedSection={activeSection}
            busyAction={busyAction}
            controlsDisabled={controlsDisabled}
            editingSettings={editingSettings}
            onRefresh={onRefresh}
            onOpenTasks={onOpenTasks}
            onBeginEdit={onBeginEdit}
            onCancelEdit={onCancelEdit}
            onSaveSettings={onSaveSettings}
          />

          <div key={activeSection} className="section-shell__content">
            {activeSection === "status" && (
              <AppShellStatusSection
                snapshot={snapshot}
                resolvedSettings={resolvedSettings}
                busyAction={busyAction}
                controlsDisabled={controlsDisabled}
                onStart={onStart}
                onStop={onStop}
                onOpenWeb={onOpenWeb}
                onOpenTasks={onOpenTasks}
                onOpenLogs={onOpenLogs}
              />
            )}

            {activeSection === "environment" && (
              <AppShellEnvironmentSection
                snapshot={snapshot}
                platformLabel={platformLabel}
              />
            )}

            {activeSection === "diagnostics" && (
              <AppShellDiagnosticsSection
                snapshot={snapshot}
                diagnosticsSummary={diagnosticsSummary}
                onOpenLogs={onOpenLogs}
              />
            )}

            {activeSection === "settings" && (
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

            {activeSection === "about" && (
              <AppShellAboutSection
                snapshot={snapshot}
                controlsDisabled={controlsDisabled}
                onCheckForUpdates={onCheckForUpdates}
                onDownloadUpdate={onDownloadUpdate}
                onInstallDownloadedUpdate={onInstallDownloadedUpdate}
                onOpenReleasePage={onOpenReleasePage}
                onOpenRepositoryPage={onOpenRepositoryPage}
              />
            )}
          </div>
        </div>
      </main>
    </div>
  );
}

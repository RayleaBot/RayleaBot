import { useDeferredValue, useEffect, useMemo, useState } from "react";
import type { LauncherResolvedSettings, LauncherSettings, LauncherSnapshot } from "@shared/launcher-models";

import { buildDiagnosticsSummary, initialSnapshot } from "./AppState.shared";

export function useLauncherSettingsState(snapshot: LauncherSnapshot, editingSettings: boolean) {
  const [editingDraft, setEditingDraft] = useState<LauncherSettings | null>(null);
  const [previewResolvedSettings, setPreviewResolvedSettings] = useState<LauncherResolvedSettings>(initialSnapshot.launcher.resolvedSettings);
  const settingsDraft = editingDraft ?? snapshot.launcher.settings;
  const deferredSettingsDraft = useDeferredValue(settingsDraft);
  const diagnosticsSummary = useMemo(() => buildDiagnosticsSummary(snapshot), [snapshot]);

  useEffect(() => {
    if (!editingSettings && editingDraft !== null) {
      setEditingDraft(null);
    }
  }, [editingSettings, editingDraft]);

  useEffect(() => {
    if (!editingSettings) {
      setPreviewResolvedSettings(snapshot.launcher.resolvedSettings);
      return;
    }
    let cancelled = false;
    const timeout = window.setTimeout(() => {
      window.rayleaLauncher.previewResolvedSettings(deferredSettingsDraft)
        .then((resolvedSettings) => {
          if (!cancelled) {
            setPreviewResolvedSettings(resolvedSettings);
          }
        })
        .catch(() => {
          if (!cancelled) {
            setPreviewResolvedSettings(snapshot.launcher.resolvedSettings);
          }
        });
    }, 150);

    return () => {
      cancelled = true;
      window.clearTimeout(timeout);
    };
  }, [editingSettings, deferredSettingsDraft, snapshot.launcher.resolvedSettings]);

  return {
    diagnosticsSummary,
    editingDraft,
    previewResolvedSettings,
    setEditingDraft,
    settingsDraft,
  };
}

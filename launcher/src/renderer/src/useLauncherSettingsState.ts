import { useDeferredValue, useEffect, useState } from "react";
import type { LauncherResolvedSettings, LauncherSettings, LauncherSnapshot } from "@shared/launcher-models";

import { buildDiagnosticsSummary, initialSnapshot } from "./AppState.shared";

export function useLauncherSettingsState(snapshot: LauncherSnapshot, editingSettings: boolean) {
  const [editingDraft, setEditingDraft] = useState<LauncherSettings | null>(null);
  const [previewResolvedSettings, setPreviewResolvedSettings] = useState<LauncherResolvedSettings>(initialSnapshot.resolvedSettings);
  const settingsDraft = editingDraft ?? snapshot.settings;
  const deferredSettingsDraft = useDeferredValue(settingsDraft);

  useEffect(() => {
    if (!editingSettings && editingDraft !== null) {
      setEditingDraft(null);
    }
  }, [snapshot.settings, editingSettings, editingDraft]);

  useEffect(() => {
    if (!editingSettings) {
      setPreviewResolvedSettings(snapshot.resolvedSettings);
      return;
    }
    let cancelled = false;
    window.rayleaLauncher.previewResolvedSettings(deferredSettingsDraft)
      .then((resolvedSettings) => {
        if (!cancelled) {
          setPreviewResolvedSettings(resolvedSettings);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setPreviewResolvedSettings(snapshot.resolvedSettings);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [editingSettings, deferredSettingsDraft, snapshot.resolvedSettings]);

  return {
    diagnosticsSummary: buildDiagnosticsSummary(snapshot),
    editingDraft,
    previewResolvedSettings,
    setEditingDraft,
    settingsDraft,
  };
}

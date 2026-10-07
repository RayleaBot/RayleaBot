import { computed, shallowRef, watch, type Ref } from "vue";
import type { LauncherResolvedSettings, LauncherSettings, LauncherSnapshot } from "@shared/launcher-models";

import { buildDiagnosticsSummary, initialSnapshot } from "./AppState.shared";

export function useLauncherSettingsState(snapshot: Readonly<Ref<LauncherSnapshot>>, editingSettings: Readonly<Ref<boolean>>) {
  const editingDraft = shallowRef<LauncherSettings | null>(null);
  const previewResolvedSettings = shallowRef<LauncherResolvedSettings>(initialSnapshot.launcher.resolvedSettings);
  const settingsDraft = computed(() => editingDraft.value ?? snapshot.value.launcher.settings);
  const diagnosticsSummary = computed(() => buildDiagnosticsSummary(snapshot.value));

  watch(editingSettings, (editing) => {
    if (!editing) {
      editingDraft.value = null;
    }
  });

  // While editing, the derived paths are previewed shortly after the draft stops changing.
  watch(
    [editingSettings, settingsDraft, () => snapshot.value.launcher.resolvedSettings],
    ([editing, draft, resolvedSettings], _previous, onCleanup) => {
      if (!editing) {
        previewResolvedSettings.value = resolvedSettings;
        return;
      }
      let cancelled = false;
      const timeout = window.setTimeout(() => {
        window.rayleaLauncher.previewResolvedSettings(draft)
          .then((preview) => {
            if (!cancelled) {
              previewResolvedSettings.value = preview;
            }
          })
          .catch(() => {
            if (!cancelled) {
              previewResolvedSettings.value = resolvedSettings;
            }
          });
      }, 150);
      onCleanup(() => {
        cancelled = true;
        window.clearTimeout(timeout);
      });
    },
    { immediate: true },
  );

  return {
    diagnosticsSummary,
    editingDraft,
    previewResolvedSettings,
    settingsDraft,
  };
}

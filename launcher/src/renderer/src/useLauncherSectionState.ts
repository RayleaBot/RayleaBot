import { readonly, ref } from "vue";

import type { SectionId } from "./AppShell.shared";
import { runLauncherWorkspaceTransition } from "./launcherMotion";

export function useLauncherSectionState() {
  const activeSection = ref<SectionId>("status");

  const setActiveSection = (nextSection: SectionId) => {
    if (nextSection === activeSection.value) return;
    runLauncherWorkspaceTransition(() => {
      activeSection.value = nextSection;
    });
  };

  return {
    activeSection: readonly(activeSection),
    setActiveSection,
  };
}

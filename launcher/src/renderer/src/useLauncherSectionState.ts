import { useCallback, useState } from "react";

import type { SectionId } from "./AppState.shared";
import { runLauncherWorkspaceTransition } from "./launcherMotion";

export function useLauncherSectionState() {
  const [activeSection, setActiveSectionState] = useState<SectionId>("status");

  const setActiveSection = useCallback((nextSection: SectionId) => {
    if (nextSection === activeSection) return;
    runLauncherWorkspaceTransition(() => setActiveSectionState(nextSection));
  }, [activeSection]);

  return {
    activeSection,
    setActiveSection,
  };
}

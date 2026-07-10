import { useCallback, useEffect, useRef, useState } from "react";

import type { SectionId, SectionTransitionState } from "./AppState.shared";

const SECTION_ENTER_MS = 200;

function prefersReducedMotion() {
  return Boolean(window.matchMedia?.("(prefers-reduced-motion: reduce)").matches);
}

export function useLauncherSectionState() {
  const [activeSection, setActiveSectionState] = useState<SectionId>("status");
  const [sectionTransitionState, setSectionTransitionState] = useState<SectionTransitionState>("idle");
  const sectionEnterTimerRef = useRef<number | null>(null);

  const clearSectionTransitionTimers = useCallback(() => {
    if (sectionEnterTimerRef.current !== null) {
      window.clearTimeout(sectionEnterTimerRef.current);
      sectionEnterTimerRef.current = null;
    }
  }, []);

  useEffect(() => clearSectionTransitionTimers, [clearSectionTransitionTimers]);

  const setActiveSection = useCallback((nextSection: SectionId) => {
    clearSectionTransitionTimers();
    setActiveSectionState(nextSection);

    if (prefersReducedMotion()) {
      setSectionTransitionState("idle");
      return;
    }

    setSectionTransitionState("entering");
    sectionEnterTimerRef.current = window.setTimeout(() => {
      setSectionTransitionState("idle");
      sectionEnterTimerRef.current = null;
    }, SECTION_ENTER_MS);
  }, [clearSectionTransitionTimers]);

  return {
    activeSection,
    renderedSection: activeSection,
    sectionTransitionState,
    setActiveSection,
  };
}

import { useCallback, useEffect, useRef, useState } from "react";

import type { SectionId, SectionTransitionState } from "./AppState.shared";

const SECTION_EXIT_MS = 90;
const SECTION_ENTER_MS = 180;

export function useLauncherSectionState() {
  const [activeSection, setActiveSection] = useState<SectionId>("status");
  const [renderedSection, setRenderedSection] = useState<SectionId>("status");
  const [sectionTransitionState, setSectionTransitionState] = useState<SectionTransitionState>("idle");
  const sectionExitTimerRef = useRef<number | null>(null);
  const sectionEnterTimerRef = useRef<number | null>(null);

  const clearSectionTransitionTimers = useCallback(() => {
    if (sectionExitTimerRef.current !== null) {
      window.clearTimeout(sectionExitTimerRef.current);
      sectionExitTimerRef.current = null;
    }
    if (sectionEnterTimerRef.current !== null) {
      window.clearTimeout(sectionEnterTimerRef.current);
      sectionEnterTimerRef.current = null;
    }
  }, []);

  useEffect(() => clearSectionTransitionTimers, [clearSectionTransitionTimers]);

  useEffect(() => {
    if (activeSection === renderedSection) {
      return;
    }
    if (sectionExitTimerRef.current !== null) {
      window.clearTimeout(sectionExitTimerRef.current);
      sectionExitTimerRef.current = null;
    }
    setSectionTransitionState("exiting");
    sectionExitTimerRef.current = window.setTimeout(() => {
      setRenderedSection(activeSection);
      setSectionTransitionState("entering");
      sectionExitTimerRef.current = null;
    }, SECTION_EXIT_MS);

    return () => {
      if (sectionExitTimerRef.current !== null) {
        window.clearTimeout(sectionExitTimerRef.current);
        sectionExitTimerRef.current = null;
      }
    };
  }, [activeSection, renderedSection]);

  useEffect(() => {
    if (sectionTransitionState !== "entering") {
      return;
    }
    if (sectionEnterTimerRef.current !== null) {
      window.clearTimeout(sectionEnterTimerRef.current);
      sectionEnterTimerRef.current = null;
    }
    sectionEnterTimerRef.current = window.setTimeout(() => {
      setSectionTransitionState("idle");
      sectionEnterTimerRef.current = null;
    }, SECTION_ENTER_MS);

    return () => {
      if (sectionEnterTimerRef.current !== null) {
        window.clearTimeout(sectionEnterTimerRef.current);
        sectionEnterTimerRef.current = null;
      }
    };
  }, [sectionTransitionState]);

  return {
    activeSection,
    renderedSection,
    sectionTransitionState,
    setActiveSection,
  };
}

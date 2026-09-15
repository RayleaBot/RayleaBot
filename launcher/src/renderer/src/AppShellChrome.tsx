import {
  Dismiss20Regular,
  Square20Regular,
  SquareMultiple20Regular,
  Subtract20Regular,
} from "@fluentui/react-icons";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { motion } from "motion/react";
import { useLayoutEffect, useRef, type MouseEvent } from "react";

import { sections, serviceStateConfig } from "./AppShell.shared";
import type { SectionId } from "./AppShell.shared";
import { useLauncherReducedMotion } from "./launcherMotion";
import { RayleaMark } from "./RayleaMark";
import { ThemeModeMenu } from "./ThemeModeMenu";

type AppShellChromeProps = {
  snapshot: LauncherSnapshot;
  activeSection: SectionId;
  isMaximized: boolean;
  onNavigate: (section: SectionId) => void;
};

const selectionTransition = { type: "spring", visualDuration: 0.32, bounce: 0.18 } as const;

export function AppShellChrome({
  snapshot,
  activeSection,
  isMaximized,
  onNavigate,
}: AppShellChromeProps) {
  const presentation = deriveLauncherPresentation(snapshot);
  const stateConfig = serviceStateConfig[presentation.state];
  const trayStatus = stateConfig.label;
  const reducedMotion = useLauncherReducedMotion();
  const selectionRef = useRef<HTMLSpanElement>(null);
  // Where the new selection starts, relative to the clicked item. It is read at click time, while the layout is
  // still clean. Motion's shared layout projection would instead read the page scroll on every sidebar render
  // and force a synchronous layout right after the workspace changes.
  const selectionStart = useRef<{ section: SectionId; offset: number } | null>(null);

  useLayoutEffect(() => {
    selectionStart.current = null;
  }, [activeSection]);

  const navigate = (event: MouseEvent<HTMLButtonElement>, section: SectionId) => {
    const previous = selectionRef.current?.getBoundingClientRect();
    if (previous && section !== activeSection) {
      selectionStart.current = { section, offset: previous.top - event.currentTarget.getBoundingClientRect().top };
    }
    onNavigate(section);
  };

  return (
    <>
      <div className="window-drag-handle">
        <div className="window-title"><RayleaMark variant="chrome" />RayleaBot 启动器</div>
        <div className="window-controls">
          <button className="window-control-btn" onClick={() => window.rayleaLauncher.minimize()} title="最小化" aria-label="最小化"><Subtract20Regular /></button>
          <button className="window-control-btn" onClick={() => window.rayleaLauncher.maximize()} title={isMaximized ? "还原" : "最大化"} aria-label={isMaximized ? "还原" : "最大化"}>{isMaximized ? <SquareMultiple20Regular /> : <Square20Regular />}</button>
          <button className="window-control-btn danger" onClick={() => window.rayleaLauncher.close()} title="关闭" aria-label="关闭"><Dismiss20Regular /></button>
        </div>
      </div>

      <aside className="shell-sidebar glass" data-glass="regular">
        <nav className="section-nav">
          {sections.map((section) => (
            <button
              key={section.id}
              className={`nav-item${activeSection === section.id ? " active" : ""}`}
              onClick={(event) => navigate(event, section.id)}
              aria-current={activeSection === section.id ? "page" : undefined}
              title={section.title}
            >
              {activeSection === section.id ? (
                <motion.span
                  ref={selectionRef}
                  className="nav-item__selection"
                  initial={!reducedMotion && selectionStart.current?.section === section.id
                    ? { y: selectionStart.current.offset }
                    : false}
                  animate={{ y: 0 }}
                  transition={selectionTransition}
                />
              ) : null}
              <span className="nav-item__icon">{section.icon}</span>
              <span className="nav-item__label">{section.title}</span>
            </button>
          ))}
        </nav>

        <div className="sidebar-footer--compact">
          <div className="sidebar-footer__status-dot" title={`运行状态：${trayStatus}`}>
            <span
              className={`status-indicator status-indicator--${stateConfig.tone}`}
              aria-label={`运行状态：${trayStatus}`}
            />
          </div>
          <ThemeModeMenu />
        </div>
      </aside>
    </>
  );
}

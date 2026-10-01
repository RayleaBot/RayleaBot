import { Button } from "@fluentui/react-components";
import {
  ArrowSync20Regular,
  Globe20Regular,
  Play20Regular,
  Power24Filled,
  Stop20Regular,
} from "@fluentui/react-icons";
import type { LauncherPresentationState } from "@shared/launcher-presentation";
import { AnimatePresence, motion, useIsPresent } from "motion/react";
import { useId, type ReactNode } from "react";

import { serviceStateConfig, serviceStateGlyphs } from "./AppShell.shared";
import { launcherMotion, useLauncherReducedMotion } from "./launcherMotion";
import { StatusLens } from "./StatusLens";

type AppShellServiceControlProps = {
  attention: {
    label: string;
    text: string;
    tone: "attention" | "warning" | "danger";
  } | null;
  busyLabel: string;
  canOpenWebUi: boolean;
  controlsDisabled: boolean;
  externalService: boolean;
  onOpenWeb: () => void;
  onStart: () => void;
  onStop: () => void;
  showRunningActions: boolean;
  stopOpensWeb: boolean;
  snapshot: {
    serviceDetail: string;
    serviceState: LauncherPresentationState;
  };
  startDisabled: boolean;
  stopDisabled: boolean;
};

/** State text that fades in as it replaces the previous copy; the outgoing copy is hidden from assistive technology. */
function StateCopy({ as = "span", className, children }: { as?: "span" | "p"; className?: string; children: ReactNode }) {
  const isPresent = useIsPresent();
  const reducedMotion = useLauncherReducedMotion();
  const Element = as === "p" ? motion.p : motion.span;
  return (
    <Element
      className={className}
      aria-hidden={isPresent ? undefined : true}
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -4, transition: { duration: reducedMotion ? 0 : 0.12, ease: launcherMotion.ease } }}
      transition={{ duration: reducedMotion ? 0 : launcherMotion.content / 1000, ease: launcherMotion.ease }}
    >
      {children}
    </Element>
  );
}

export function AppShellServiceControl({
  attention,
  busyLabel,
  canOpenWebUi,
  controlsDisabled,
  externalService,
  onOpenWeb,
  onStart,
  onStop,
  showRunningActions,
  snapshot,
  startDisabled,
  stopDisabled,
  stopOpensWeb,
}: AppShellServiceControlProps) {
  const noteId = useId();
  const stateConfig = serviceStateConfig[snapshot.serviceState];
  const tone = stateConfig?.tone ?? "neutral";
  const stateLabel = stateConfig?.label ?? "未知";
  const primaryDisabled = showRunningActions ? !canOpenWebUi || controlsDisabled : startDisabled;
  const note = busyLabel
    || (!showRunningActions
      ? "服务启动后可进入管理界面"
      : externalService
        ? "服务由其他进程启动，无法在启动器中重启"
        : "");

  return (
    <section className="service-control" data-tone={tone} aria-labelledby="service-control-title">
      <div className="service-control__summary">
        <div className="service-control__state" aria-live="polite">
          <StatusLens
            tone={tone}
            icon={serviceStateGlyphs[snapshot.serviceState] ?? <Power24Filled />}
            iconKey={snapshot.serviceState}
            spinning={snapshot.serviceState === "starting" || snapshot.serviceState === "stopping"}
          />
          <div className="service-control__state-copy">
            <h2 id="service-control-title" className="service-control__state-value">
              <span className="visually-hidden">服务控制：</span>
              <span className="presence-stack">
                <AnimatePresence initial={false}>
                  <StateCopy key={stateLabel}>{stateLabel}</StateCopy>
                </AnimatePresence>
              </span>
            </h2>
            <div className="presence-stack">
              <AnimatePresence initial={false}>
                <StateCopy key={snapshot.serviceDetail} as="p" className="service-control__detail">{snapshot.serviceDetail}</StateCopy>
              </AnimatePresence>
            </div>
          </div>
        </div>
        {attention ? (
          <div className="attention-note" data-severity={attention.tone}>
            <span className="attention-note__label">{attention.label}</span>
            <span>{attention.text}</span>
          </div>
        ) : null}
      </div>

      <div className="service-control__actions">
        {/* A service started elsewhere can only be stopped from its own management console. */}
        {stopOpensWeb ? (
          <Button appearance="secondary" className="launcher-button" data-emphasis="regular" onClick={onOpenWeb} disabled={controlsDisabled || !canOpenWebUi} icon={<Globe20Regular />}>在管理界面停止</Button>
        ) : (
          <Button appearance="secondary" className="launcher-button launcher-button--danger" data-emphasis="regular" onClick={onStop} disabled={stopDisabled} icon={<Stop20Regular />}>停止服务</Button>
        )}
        {showRunningActions ? (
          <Button
            appearance="secondary"
            className="launcher-button"
            data-emphasis="regular"
            onClick={onStart}
            disabled={startDisabled}
            aria-describedby={externalService ? noteId : undefined}
            icon={<ArrowSync20Regular />}
          >
            重启服务
          </Button>
        ) : (
          <Button appearance="secondary" className="launcher-button" data-emphasis="regular" onClick={onOpenWeb} disabled icon={<Globe20Regular />}>管理界面</Button>
        )}
        <Button
          appearance="primary"
          className="launcher-button service-control__primary"
          data-emphasis={primaryDisabled ? "regular" : "prominent"}
          onClick={showRunningActions ? onOpenWeb : onStart}
          disabled={primaryDisabled}
          icon={showRunningActions ? <Globe20Regular /> : <Play20Regular />}
        >
          {showRunningActions ? "管理界面" : "启动服务"}
        </Button>
        {note ? (
          <p id={noteId} className="operation-status" aria-live="polite">
            {note}
          </p>
        ) : null}
      </div>
    </section>
  );
}

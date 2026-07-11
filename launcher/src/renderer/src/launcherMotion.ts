import { flushSync } from "react-dom";

export const launcherMotion = {
  control: 160,
  content: 200,
  workspace: 300,
  workspaceEase: "cubic-bezier(0.25, 0.1, 0.25, 1)",
  overlay: 220,
  ease: "cubic-bezier(0.16, 1, 0.3, 1)",
} as const;

export type LauncherViewTransitionKind = "theme";

interface LauncherViewTransitionRequest {
  sequence: number;
  kind: LauncherViewTransitionKind;
  update: () => void;
}

interface ActiveLauncherViewTransition {
  request: LauncherViewTransitionRequest;
  skipRequested: boolean;
  transition: ViewTransition;
}

let activeViewTransition: ActiveLauncherViewTransition | null = null;
let pendingViewTransition: LauncherViewTransitionRequest | null = null;
let transitionSequence = 0;

interface ActiveWorkspaceAnimation {
  animation: Animation;
  fromOpacity: number;
}

let activeWorkspaceAnimation: ActiveWorkspaceAnimation | null = null;
const workspaceEntryOpacity = 0;

export function prefersReducedMotion(): boolean {
  return typeof window !== "undefined" &&
    Boolean(window.matchMedia?.("(prefers-reduced-motion: reduce)").matches);
}

export function supportsViewTransitions(): boolean {
  return typeof document !== "undefined" &&
    typeof document.startViewTransition === "function";
}

function cancelWorkspaceAnimation(): number {
  const active = activeWorkspaceAnimation;
  if (!active) return workspaceEntryOpacity;

  const progress = active.animation.effect?.getComputedTiming().progress;
  const opacity = typeof progress === "number"
    ? active.fromOpacity + ((1 - active.fromOpacity) * progress)
    : workspaceEntryOpacity;
  activeWorkspaceAnimation = null;
  active.animation.cancel();
  return opacity;
}

export function runLauncherWorkspaceTransition(update: () => void): Animation | null {
  const fromOpacity = cancelWorkspaceAnimation();
  const workspace = typeof document === "undefined"
    ? null
    : document.querySelector<HTMLElement>(".shell-main");

  if (
    prefersReducedMotion()
    || !workspace
    || typeof workspace.animate !== "function"
  ) {
    update();
    return null;
  }

  flushSync(update);
  const animation = workspace.animate(
    [
      { opacity: fromOpacity },
      { opacity: 1 },
    ],
    {
      duration: launcherMotion.workspace,
      easing: launcherMotion.workspaceEase,
      fill: "both",
    },
  );
  const active = { animation, fromOpacity } satisfies ActiveWorkspaceAnimation;
  activeWorkspaceAnimation = active;
  void animation.finished.catch(() => undefined).finally(() => {
    if (activeWorkspaceAnimation !== active) return;
    activeWorkspaceAnimation = null;
    animation.cancel();
  });
  return animation;
}

function startLauncherViewTransition(
  request: LauncherViewTransitionRequest,
): ViewTransition | null {
  const root = document.documentElement;
  root.dataset.launcherViewTransitionKind = request.kind;

  let transition: ViewTransition;
  try {
    transition = document.startViewTransition(() => {
      if (request.sequence !== transitionSequence) return;
      flushSync(request.update);
    });
  } catch {
    if (request.sequence === transitionSequence) {
      request.update();
    }
    delete root.dataset.launcherViewTransitionKind;
    return null;
  }

  const active = {
    request,
    skipRequested: false,
    transition,
  } satisfies ActiveLauncherViewTransition;
  activeViewTransition = active;
  void transition.finished.catch(() => undefined).finally(() => {
    if (activeViewTransition !== active) return;
    activeViewTransition = null;

    const pending = pendingViewTransition;
    pendingViewTransition = null;
    if (pending && pending.sequence === transitionSequence) {
      startLauncherViewTransition(pending);
      return;
    }

    delete root.dataset.launcherViewTransitionKind;
  });
  return transition;
}

export function runLauncherViewTransition(
  kind: LauncherViewTransitionKind,
  update: () => void,
): ViewTransition | null {
  const request = {
    sequence: ++transitionSequence,
    kind,
    update,
  } satisfies LauncherViewTransitionRequest;

  if (!supportsViewTransitions() || prefersReducedMotion()) {
    pendingViewTransition = null;
    activeViewTransition?.transition.skipTransition();
    activeViewTransition = null;
    if (typeof document !== "undefined") {
      delete document.documentElement.dataset.launcherViewTransitionKind;
    }
    update();
    return null;
  }

  if (activeViewTransition) {
    pendingViewTransition = request;
    if (!activeViewTransition.skipRequested) {
      activeViewTransition.skipRequested = true;
      activeViewTransition.transition.skipTransition();
    }
    return null;
  }

  return startLauncherViewTransition(request);
}

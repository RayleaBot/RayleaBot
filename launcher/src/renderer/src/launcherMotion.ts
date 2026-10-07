import { nextTick, onScopeDispose, readonly, ref, type Ref } from "vue";
import { animate, motionValue } from "motion-v";

export const launcherMotion = {
  control: 160,
  content: 200,
  workspace: 220,
  workspaceEase: [0.25, 0.1, 0.25, 1],
  overlay: 220,
  overlayExit: 160,
  themeReveal: 420,
  themeFade: 280,
  ease: [0.16, 1, 0.3, 1],
} as const;

/** Overlays arrive at the overlay duration and leave faster, both on the launcher's decelerating curve. */
export const overlayEnter = { duration: launcherMotion.overlay / 1000, ease: launcherMotion.ease };
export const overlayExit = { duration: launcherMotion.overlayExit / 1000, ease: launcherMotion.ease };

const reducedMotionQuery = "(prefers-reduced-motion: reduce), (forced-colors: active)";

/** Viewport point the new theme grows out of, usually the center of the theme button. */
export type ThemeMotionOrigin = { x: number; y: number };

type LauncherViewTransitionKind = "theme";

interface LauncherViewTransitionRequest {
  sequence: number;
  kind: LauncherViewTransitionKind;
  update: () => void;
  origin?: ThemeMotionOrigin;
}

interface ActiveLauncherViewTransition {
  request: LauncherViewTransitionRequest;
  skipRequested: boolean;
  transition: ViewTransition;
  reveal: ReturnType<typeof animate> | null;
}

let activeViewTransition: ActiveLauncherViewTransition | null = null;
let pendingViewTransition: LauncherViewTransitionRequest | null = null;
let transitionSequence = 0;

/** Opacity of the workspace. A newly shown workspace fades in from a slightly lowered opacity. */
export const workspaceOpacity = motionValue(1);
const workspaceEntryOpacity = 0.88;
let workspaceAnimation: ReturnType<typeof animate> | null = null;

function prefersReducedMotion(): boolean {
  return typeof window !== "undefined" &&
    Boolean(window.matchMedia?.(reducedMotionQuery).matches);
}

/** Reduced motion and forced colors, as a live value for Motion components. */
export function useLauncherReducedMotion(): Readonly<Ref<boolean>> {
  const reduced = ref(prefersReducedMotion());
  const query = typeof window === "undefined" ? undefined : window.matchMedia?.(reducedMotionQuery);
  const update = () => {
    reduced.value = Boolean(query?.matches);
  };
  query?.addEventListener?.("change", update);
  onScopeDispose(() => query?.removeEventListener?.("change", update));
  return readonly(reduced);
}

function supportsViewTransitions(): boolean {
  return typeof document !== "undefined" &&
    typeof document.startViewTransition === "function";
}

/** Vue renders the update before the next frame, so the fade starts on the new workspace. */
export function runLauncherWorkspaceTransition(update: () => void): void {
  const interrupted = workspaceAnimation !== null;
  workspaceAnimation?.stop();
  workspaceAnimation = null;

  update();
  if (prefersReducedMotion()) {
    workspaceOpacity.jump(1);
    return;
  }

  // An interrupted fade continues from its current opacity instead of dropping back to the entry opacity.
  workspaceOpacity.jump(interrupted ? workspaceOpacity.get() : workspaceEntryOpacity);
  const controls = animate(workspaceOpacity, 1, {
    duration: launcherMotion.workspace / 1000,
    ease: launcherMotion.workspaceEase,
  });
  workspaceAnimation = controls;
  void controls.then(() => {
    if (workspaceAnimation === controls) workspaceAnimation = null;
  });
}

/**
 * Grows the new theme's snapshot out of the origin as a circle that reaches the farthest window corner, or
 * fades it in when there is no origin. The old snapshot stays underneath until it is covered.
 */
function revealTheme(origin: ThemeMotionOrigin | undefined): ReturnType<typeof animate> | null {
  const root = document.documentElement;
  if (typeof root.animate !== "function") return null;
  const options = { ease: launcherMotion.ease, pseudoElement: "::view-transition-new(root)" };
  if (!origin) {
    return animate(root, { opacity: [0, 1] }, { ...options, duration: launcherMotion.themeFade / 1000 });
  }
  const width = window.innerWidth;
  const height = window.innerHeight;
  const x = Math.max(0, Math.min(width, origin.x));
  const y = Math.max(0, Math.min(height, origin.y));
  const radius = Math.hypot(Math.max(x, width - x), Math.max(y, height - y));
  return animate(
    root,
    { clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${radius}px at ${x}px ${y}px)`] },
    { ...options, duration: launcherMotion.themeReveal / 1000 },
  );
}

function skipActiveViewTransition(active: ActiveLauncherViewTransition) {
  active.reveal?.cancel();
  active.reveal = null;
  active.transition.skipTransition();
}

function startLauncherViewTransition(
  request: LauncherViewTransitionRequest,
): ViewTransition | null {
  const root = document.documentElement;
  root.dataset.launcherViewTransitionKind = request.kind;

  let transition: ViewTransition;
  try {
    // The new snapshot is captured once Vue has rendered the update.
    transition = document.startViewTransition(() => {
      if (request.sequence !== transitionSequence) return;
      request.update();
      return nextTick();
    });
  } catch {
    if (request.sequence === transitionSequence) {
      request.update();
    }
    delete root.dataset.launcherViewTransitionKind;
    return null;
  }

  const active: ActiveLauncherViewTransition = {
    request,
    skipRequested: false,
    transition,
    reveal: null,
  };
  activeViewTransition = active;
  void transition.ready.then(() => {
    if (activeViewTransition === active && !active.skipRequested) {
      active.reveal = revealTheme(request.origin);
    }
  }).catch(() => undefined);
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
  origin?: ThemeMotionOrigin,
): ViewTransition | null {
  const request = {
    sequence: ++transitionSequence,
    kind,
    update,
    origin,
  } satisfies LauncherViewTransitionRequest;

  if (!supportsViewTransitions() || prefersReducedMotion()) {
    pendingViewTransition = null;
    if (activeViewTransition) skipActiveViewTransition(activeViewTransition);
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
      skipActiveViewTransition(activeViewTransition);
    }
    return null;
  }

  return startLauncherViewTransition(request);
}

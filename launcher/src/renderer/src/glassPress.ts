/**
 * Press feedback for glass controls: a control sinks slightly while it is held and springs back when released,
 * as Liquid Glass controls do. The listeners are delegated from the document, so controls inside portals and
 * controls mounted later respond without wrapping every Fluent button.
 */
import { animate } from "motion/react";

import { launcherMotion, prefersReducedMotion } from "./launcherMotion";

const pressableGlass = [
  ".glass-button",
  ".service-control__primary",
  ".glass-dialog__button",
  ".glass-tile",
  ".theme-menu-trigger",
].join(", ");

const pressTransition = { duration: 0.12, ease: launcherMotion.ease };
const releaseTransition = { type: "spring", visualDuration: 0.34, bounce: 0.42 } as const;

// Wide option tiles sink less, so their edges do not travel further than a capsule's.
const pressedScale = (element: HTMLElement) => (element.classList.contains("glass-tile") ? 0.985 : 0.96);

export function startGlassPress(): () => void {
  let pressed: HTMLElement | null = null;

  const release = () => {
    const element = pressed;
    pressed = null;
    if (element) animate(element, { scale: 1 }, releaseTransition);
  };

  const press = (target: EventTarget | null) => {
    if (!(target instanceof Element) || prefersReducedMotion()) return;
    const element = target.closest<HTMLElement>(pressableGlass);
    if (!element || element === pressed || element.matches(":disabled, [aria-disabled='true']")) return;
    release();
    pressed = element;
    animate(element, { scale: pressedScale(element) }, pressTransition);
  };

  const onPointerDown = (event: PointerEvent) => {
    if (event.button === 0) press(event.target);
  };
  const onKeyDown = (event: KeyboardEvent) => {
    if (!event.repeat && (event.key === " " || event.key === "Enter")) press(event.target);
  };

  document.addEventListener("pointerdown", onPointerDown, true);
  document.addEventListener("keydown", onKeyDown, true);
  window.addEventListener("pointerup", release, true);
  window.addEventListener("pointercancel", release, true);
  window.addEventListener("keyup", release, true);
  window.addEventListener("blur", release);

  return () => {
    document.removeEventListener("pointerdown", onPointerDown, true);
    document.removeEventListener("keydown", onKeyDown, true);
    window.removeEventListener("pointerup", release, true);
    window.removeEventListener("pointercancel", release, true);
    window.removeEventListener("keyup", release, true);
    window.removeEventListener("blur", release);
    pressed = null;
  };
}

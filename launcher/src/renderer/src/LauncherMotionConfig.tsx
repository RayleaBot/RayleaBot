import { MotionConfig } from "motion/react";
import type { ReactNode } from "react";

import { useLauncherReducedMotion } from "./launcherMotion";

/** Motion follows the launcher's reduced-motion preference, which also covers forced colors. */
export function LauncherMotionConfig({ children }: { children: ReactNode }) {
  const reducedMotion = useLauncherReducedMotion();
  return <MotionConfig reducedMotion={reducedMotion ? "always" : "never"}>{children}</MotionConfig>;
}

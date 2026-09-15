import { AnimatePresence, motion } from "motion/react";
import type { ReactNode } from "react";

import type { LauncherVisualTone } from "./AppShell.shared";
import { launcherMotion, useLauncherReducedMotion } from "./launcherMotion";

type StatusLensProps = {
  tone: LauncherVisualTone;
  icon: ReactNode;
  size?: "regular" | "small";
  /** Changing the key swaps the glyph with a short scale and fade. */
  iconKey?: string;
  /** Turns the glyph slowly while an operation is in progress. */
  spinning?: boolean;
};

export function StatusLens({ tone, icon, size = "regular", iconKey = "glyph", spinning = false }: StatusLensProps) {
  const reducedMotion = useLauncherReducedMotion();
  const turning = spinning && !reducedMotion;

  return (
    <span className={size === "small" ? "status-lens status-lens--small" : "status-lens"} data-tone={tone} aria-hidden="true">
      <span className="status-lens__core" />
      <span className="status-lens__glass glass" data-glass="clear">
        <AnimatePresence initial={false}>
          <motion.span
            key={iconKey}
            className="status-lens__glyph"
            initial={{ opacity: 0, scale: 0.6 }}
            animate={{ opacity: 1, scale: 1, rotate: turning ? 360 : 0 }}
            exit={{ opacity: 0, scale: 0.6 }}
            transition={reducedMotion
              ? { duration: 0 }
              : {
                duration: launcherMotion.content / 1000,
                ease: launcherMotion.ease,
                rotate: turning ? { duration: 1.4, ease: "linear", repeat: Infinity } : { duration: 0 },
              }}
          >
            {icon}
          </motion.span>
        </AnimatePresence>
      </span>
    </span>
  );
}

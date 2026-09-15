import { Dialog, DialogSurface } from "@fluentui/react-components";
import { motion, type HTMLMotionProps } from "motion/react";
import type { ElementType, HTMLAttributes, ReactNode } from "react";

import { overlayEnter, overlayExit, useExitPresence, useLauncherReducedMotion } from "./launcherMotion";

const MotionDialogSurface = motion.create(DialogSurface);

type GlassDialogProps = {
  open: boolean;
  onDismiss: () => void;
  tone?: "danger" | "attention";
  children: ReactNode;
};

/**
 * A Fluent dialog whose glass surface and backdrop are animated by Motion. The dialog, its content and its
 * focus trap stay mounted until the exit animation ends, so the glass does not vanish mid-fade.
 */
export function GlassDialog({ open, onDismiss, tone, children }: GlassDialogProps) {
  const [present, finishExit] = useExitPresence(open);
  const reducedMotion = useLauncherReducedMotion();
  const transition = reducedMotion ? { duration: 0 } : open ? overlayEnter : overlayExit;

  return (
    <Dialog
      open={present}
      surfaceMotion={null}
      onOpenChange={(_event, data) => {
        if (!data.open) onDismiss();
      }}
    >
      <MotionDialogSurface
        className="glass-dialog"
        data-tone={tone}
        data-state={open ? "open" : "closing"}
        backdropMotion={null}
        backdrop={{
          children: (_Component: ElementType, props: HTMLAttributes<HTMLDivElement>) => (
            <motion.div
              {...(props as HTMLMotionProps<"div">)}
              initial={reducedMotion ? false : { opacity: 0 }}
              animate={{ opacity: open ? 1 : 0 }}
              transition={transition}
            />
          ),
        }}
        initial={reducedMotion ? false : { opacity: 0, scale: 0.96 }}
        animate={open ? { opacity: 1, scale: 1 } : { opacity: 0, scale: 0.96 }}
        transition={transition}
        onAnimationComplete={finishExit}
      >
        {children}
      </MotionDialogSurface>
    </Dialog>
  );
}

import { ChevronRight16Regular } from "@fluentui/react-icons";
import { motion } from "motion/react";
import { useLayoutEffect, useRef, useState, type MouseEvent, type ReactNode } from "react";

import { launcherMotion, overlayExit, useLauncherReducedMotion } from "./launcherMotion";

type DisclosureProps = {
  className: string;
  title: ReactNode;
  meta: ReactNode;
  children: ReactNode;
};

/**
 * A native details element whose content expands and collapses with Motion. The element stays open until the
 * collapse finishes, and its content stays in the document while collapsed, as with a plain details element.
 */
export function Disclosure({ className, title, meta, children }: DisclosureProps) {
  const reducedMotion = useLauncherReducedMotion();
  const [open, setOpen] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const expandedRef = useRef(expanded);
  useLayoutEffect(() => {
    expandedRef.current = expanded;
  }, [expanded]);

  const toggle = (event: MouseEvent<HTMLElement>) => {
    event.preventDefault();
    if (expanded) {
      setExpanded(false);
      if (reducedMotion) setOpen(false);
      return;
    }
    setOpen(true);
    setExpanded(true);
  };

  return (
    <details className={className} open={open} data-expanded={expanded ? "true" : "false"}>
      <summary onClick={toggle}>
        <ChevronRight16Regular className="disclosure__chevron" aria-hidden="true" />
        <span>{title}</span>
        <span>{meta}</span>
      </summary>
      <motion.div
        className="disclosure__content"
        initial={false}
        animate={expanded ? { height: "auto", opacity: 1 } : { height: 0, opacity: 0 }}
        transition={reducedMotion
          ? { duration: 0 }
          : expanded
            ? { duration: launcherMotion.overlay / 1000, ease: launcherMotion.ease }
            : overlayExit}
        onAnimationComplete={() => {
          if (!expandedRef.current) setOpen(false);
        }}
      >
        {children}
      </motion.div>
    </details>
  );
}

import {
  useEffect,
  useRef,
  useState,
  type AnimationEvent,
  type ReactElement,
} from "react";
import {
  Menu,
  MenuButton,
  MenuItemRadio,
  MenuList,
  MenuPopover,
  MenuTrigger,
} from "@fluentui/react-components";
import {
  Desktop20Regular,
  WeatherMoon20Regular,
  WeatherSunny20Regular,
} from "@fluentui/react-icons";
import type { LauncherThemeMode } from "@shared/launcher-theme";
import { useTheme } from "./useTheme";

const modeConfig: Record<LauncherThemeMode, { icon: ReactElement; label: string }> = {
  system: { icon: <Desktop20Regular />, label: "跟随系统" },
  light: { icon: <WeatherSunny20Regular />, label: "浅色" },
  dark: { icon: <WeatherMoon20Regular />, label: "深色" },
};

const menuExitFallbackMs = 180;

function prefersReducedMotion(): boolean {
  return typeof window !== "undefined" &&
    Boolean(window.matchMedia?.("(prefers-reduced-motion: reduce)").matches);
}

export function ThemeModeMenu() {
  const { mode, setMode, syncError } = useTheme();
  const [open, setOpen] = useState(false);
  const [closing, setClosing] = useState(false);
  const [pendingMode, setPendingMode] = useState<LauncherThemeMode | null>(null);
  const closeTimer = useRef<number | null>(null);
  const commitTimer = useRef<number | null>(null);
  const pendingModeRef = useRef<LauncherThemeMode | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  const finishClose = (deferThemeChange = true) => {
    if (closeTimer.current !== null) {
      window.clearTimeout(closeTimer.current);
    }
    closeTimer.current = null;

    const nextMode = pendingModeRef.current;
    pendingModeRef.current = null;
    setOpen(false);
    setClosing(false);
    setPendingMode(null);
    triggerRef.current?.focus();

    if (nextMode === null || nextMode === mode) {
      return;
    }

    if (!deferThemeChange) {
      setMode(nextMode);
      return;
    }

    // Commit on the next task so the popover is removed before its provider
    // changes theme. This keeps the exit animation visually continuous.
    commitTimer.current = window.setTimeout(() => {
      commitTimer.current = null;
      setMode(nextMode);
    }, 0);
  };

  const requestClose = (nextMode?: LauncherThemeMode) => {
    if (nextMode !== undefined) {
      pendingModeRef.current = nextMode;
      setPendingMode(nextMode);
    }

    if (!open || closing || closeTimer.current !== null) {
      return;
    }
    if (prefersReducedMotion()) {
      finishClose(false);
      return;
    }
    setClosing(true);
    closeTimer.current = window.setTimeout(finishClose, menuExitFallbackMs);
  };

  useEffect(() => () => {
    if (closeTimer.current !== null) {
      window.clearTimeout(closeTimer.current);
    }
    if (commitTimer.current !== null) {
      window.clearTimeout(commitTimer.current);
    }
  }, []);

  const selectMode = (nextMode: LauncherThemeMode) => {
    requestClose(nextMode);
  };

  const handleSurfaceAnimationEnd = (event: AnimationEvent<HTMLDivElement>) => {
    if (closing && event.currentTarget === event.target) {
      finishClose();
    }
  };

  return (
    <Menu
      open={open}
      checkedValues={{ theme: [pendingMode ?? mode] }}
      onOpenChange={(_event, data) => {
        if (data.open) {
          if (closeTimer.current !== null) {
            window.clearTimeout(closeTimer.current);
            closeTimer.current = null;
          }
          if (commitTimer.current !== null) {
            window.clearTimeout(commitTimer.current);
            commitTimer.current = null;
          }
          pendingModeRef.current = null;
          setPendingMode(null);
          setClosing(false);
          setOpen(true);
        } else {
          requestClose();
        }
      }}
      positioning={{ position: "above", align: "start" }}
    >
      <MenuTrigger disableButtonEnhancement>
        <MenuButton
          ref={triggerRef}
          className="sidebar-icon-btn theme-menu-trigger"
          icon={modeConfig[mode].icon}
          aria-label={`主题：${modeConfig[mode].label}`}
          title={`主题：${modeConfig[mode].label}`}
        />
      </MenuTrigger>
      <MenuPopover className="theme-menu-positioner">
        <div
          className="theme-menu-surface"
          data-state={closing ? "closing" : "open"}
          onAnimationEnd={handleSurfaceAnimationEnd}
        >
          <MenuList aria-label="选择主题">
            {(Object.keys(modeConfig) as LauncherThemeMode[]).map((itemMode) => (
              <MenuItemRadio
                key={itemMode}
                className="theme-menu-item"
                name="theme"
                value={itemMode}
                icon={modeConfig[itemMode].icon}
                onClick={() => selectMode(itemMode)}
              >
                {modeConfig[itemMode].label}
              </MenuItemRadio>
            ))}
          </MenuList>
          {syncError ? <p className="theme-menu-error" role="status">{syncError}</p> : null}
        </div>
      </MenuPopover>
    </Menu>
  );
}

import {
  useLayoutEffect,
  useRef,
  useState,
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
import { motion } from "motion/react";
import type { LauncherThemeMode } from "@shared/launcher-theme";
import { overlayEnter, overlayExit, prefersReducedMotion, useLauncherReducedMotion } from "./launcherMotion";
import { useTheme } from "./useTheme";

const modeConfig: Record<LauncherThemeMode, { icon: ReactElement; label: string }> = {
  system: { icon: <Desktop20Regular />, label: "跟随系统" },
  light: { icon: <WeatherSunny20Regular />, label: "浅色" },
  dark: { icon: <WeatherMoon20Regular />, label: "深色" },
};

export function ThemeModeMenu() {
  const { mode, setMode, syncError } = useTheme();
  const reducedMotion = useLauncherReducedMotion();
  const [open, setOpen] = useState(false);
  const [surfaceVisible, setSurfaceVisible] = useState(false);
  const [pendingMode, setPendingMode] = useState<LauncherThemeMode | null>(null);
  const pendingModeRef = useRef<LauncherThemeMode | null>(null);
  const surfaceVisibleRef = useRef(false);
  const triggerRef = useRef<HTMLButtonElement>(null);

  useLayoutEffect(() => {
    surfaceVisibleRef.current = surfaceVisible;
  }, [surfaceVisible]);

  const finishClose = () => {
    const nextMode = pendingModeRef.current;
    pendingModeRef.current = null;
    setOpen(false);
    setSurfaceVisible(false);
    setPendingMode(null);
    triggerRef.current?.focus();

    if (nextMode === null || nextMode === mode) {
      return;
    }
    setMode(nextMode);
  };

  const requestClose = (nextMode?: LauncherThemeMode) => {
    if (nextMode !== undefined) {
      pendingModeRef.current = nextMode;
      setPendingMode(nextMode);
    }

    if (!open || !surfaceVisible) {
      return;
    }
    if (prefersReducedMotion()) {
      finishClose();
      return;
    }
    setSurfaceVisible(false);
  };

  const selectMode = (nextMode: LauncherThemeMode) => {
    requestClose(nextMode);
  };

  return (
    <Menu
      open={open}
      // Fluent's default surface motion leaves an opacity animation on the popover, which makes the popover the
      // backdrop root so the glass surface can no longer blur the window. Motion animates the surface instead.
      surfaceMotion={null}
      checkedValues={{ theme: [pendingMode ?? mode] }}
      onOpenChange={(_event, data) => {
        if (data.open) {
          pendingModeRef.current = null;
          setPendingMode(null);
          setOpen(true);
          setSurfaceVisible(true);
        } else {
          requestClose();
        }
      }}
      positioning={{ position: "above", align: "start" }}
    >
      <MenuTrigger disableButtonEnhancement>
        <MenuButton
          ref={triggerRef}
          className="theme-menu-trigger"
          icon={modeConfig[mode].icon}
          aria-label={`主题：${modeConfig[mode].label}`}
          title={`主题：${modeConfig[mode].label}`}
        />
      </MenuTrigger>
      <MenuPopover className="theme-menu-positioner">
        <motion.div
          className="theme-menu-surface"
          data-state={surfaceVisible ? "open" : "closing"}
          initial={reducedMotion ? false : { opacity: 0, y: 5 }}
          animate={surfaceVisible ? { opacity: 1, y: 0 } : { opacity: 0, y: 3 }}
          transition={reducedMotion ? { duration: 0 } : surfaceVisible ? overlayEnter : overlayExit}
          onAnimationComplete={() => {
            if (!surfaceVisibleRef.current) finishClose();
          }}
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
        </motion.div>
      </MenuPopover>
    </Menu>
  );
}

import {
  useRef,
  useState,
  type ReactElement,
} from "react";
import { createPresenceComponent } from "@fluentui/react-motion";
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
import { launcherMotion, prefersReducedMotion } from "./launcherMotion";
import { useTheme } from "./useTheme";

const modeConfig: Record<LauncherThemeMode, { icon: ReactElement; label: string }> = {
  system: { icon: <Desktop20Regular />, label: "跟随系统" },
  light: { icon: <WeatherSunny20Regular />, label: "浅色" },
  dark: { icon: <WeatherMoon20Regular />, label: "深色" },
};

const ThemeMenuPresence = createPresenceComponent({
  enter: {
    keyframes: [
      { opacity: 0, transform: "translateY(5px)" },
      { opacity: 1, transform: "translateY(0)" },
    ],
    duration: launcherMotion.overlay,
    easing: launcherMotion.ease,
  },
  exit: {
    keyframes: [
      { opacity: 1, transform: "translateY(0)" },
      { opacity: 0, transform: "translateY(3px)" },
    ],
    duration: launcherMotion.overlay,
    easing: launcherMotion.ease,
  },
});

export function ThemeModeMenu() {
  const { mode, setMode, syncError } = useTheme();
  const [open, setOpen] = useState(false);
  const [surfaceVisible, setSurfaceVisible] = useState(false);
  const [pendingMode, setPendingMode] = useState<LauncherThemeMode | null>(null);
  const pendingModeRef = useRef<LauncherThemeMode | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

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

  const handleSurfaceMotionFinish = (_event: null, data: { direction: "enter" | "exit" }) => {
    if (data.direction === "exit") {
      finishClose();
    }
  };

  return (
    <Menu
      open={open}
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
          className="sidebar-icon-btn theme-menu-trigger"
          icon={modeConfig[mode].icon}
          aria-label={`主题：${modeConfig[mode].label}`}
          title={`主题：${modeConfig[mode].label}`}
        />
      </MenuTrigger>
      <MenuPopover className="theme-menu-positioner">
        <ThemeMenuPresence
          appear
          visible={surfaceVisible}
          unmountOnExit
          onMotionFinish={handleSurfaceMotionFinish}
        >
          <div className="theme-menu-surface" data-state={surfaceVisible ? "open" : "closing"}>
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
        </ThemeMenuPresence>
      </MenuPopover>
    </Menu>
  );
}

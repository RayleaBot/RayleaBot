import { useRef, type ReactElement } from "react";
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
import { useLauncherReducedMotion } from "./launcherMotion";
import { useTheme } from "./useTheme";

const modeConfig: Record<LauncherThemeMode, { icon: ReactElement; label: string }> = {
  system: { icon: <Desktop20Regular />, label: "跟随系统" },
  light: { icon: <WeatherSunny20Regular />, label: "浅色" },
  dark: { icon: <WeatherMoon20Regular />, label: "深色" },
};

/**
 * The theme menu opens with Fluent's own menu motion and closes as soon as an item is chosen. Fluent returns
 * focus to the trigger, and the new theme grows out of the trigger's center.
 */
export function ThemeModeMenu() {
  const { mode, setMode, syncError } = useTheme();
  // Fluent's motion follows prefers-reduced-motion only; the launcher also stills it in forced colors.
  const reducedMotion = useLauncherReducedMotion();
  const triggerRef = useRef<HTMLButtonElement>(null);

  const selectMode = (nextMode: LauncherThemeMode) => {
    if (nextMode === mode) {
      return;
    }
    const bounds = triggerRef.current?.getBoundingClientRect();
    setMode(nextMode, bounds ? { x: bounds.left + bounds.width / 2, y: bounds.top + bounds.height / 2 } : undefined);
  };

  return (
    <Menu
      checkedValues={{ theme: [mode] }}
      surfaceMotion={reducedMotion ? null : undefined}
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
        <div className="theme-menu-surface">
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

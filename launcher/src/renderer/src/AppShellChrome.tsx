import {
  ArrowClockwise20Regular,
  Dismiss20Regular,
  Square20Regular,
  SquareMultiple20Regular,
  Subtract20Regular,
  WeatherMoon20Regular,
  WeatherSunny20Regular,
  Desktop20Regular,
} from "@fluentui/react-icons";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type { LauncherSnapshot } from "@shared/launcher-models";

import { sections, serviceStateConfig, statusSummary } from "./AppShell.shared";
import type { SectionId } from "./AppShell.shared";
import { useTheme, type ThemeMode } from "./useTheme";

type AppShellChromeProps = {
  snapshot: LauncherSnapshot;
  activeSection: SectionId;
  isMaximized: boolean;
  onNavigate: (section: SectionId) => void;
  onRefresh: () => void;
};

const modeIcons: Record<ThemeMode, React.ReactNode> = {
  light: <WeatherSunny20Regular />,
  dark: <WeatherMoon20Regular />,
  system: <Desktop20Regular />,
};

const modeLabels: Record<ThemeMode, string> = {
  light: "浅色",
  dark: "深色",
  system: "跟随系统",
};

export function AppShellChrome({
  snapshot,
  activeSection,
  isMaximized,
  onNavigate,
  onRefresh,
}: AppShellChromeProps) {
  const trayStatus = statusSummary(deriveLauncherPresentation(snapshot).state);
  const { mode, toggleMode } = useTheme();

  return (
    <>
      <div className="window-drag-handle">
        <div className="window-title">RAYLEALAUNCHER</div>
        <div className="window-controls">
          <button className="window-control-btn" onClick={() => window.rayleaLauncher.minimize()} title="最小化"><Subtract20Regular /></button>
          <button className="window-control-btn" onClick={() => window.rayleaLauncher.maximize()} title={isMaximized ? "还原" : "最大化"}>{isMaximized ? <SquareMultiple20Regular /> : <Square20Regular />}</button>
          <button className="window-control-btn danger" onClick={() => window.rayleaLauncher.close()} title="关闭"><Dismiss20Regular /></button>
        </div>
      </div>

      <aside className="shell-sidebar">
        <nav className="section-nav">
          {sections.map((section) => (
            <button
              key={section.id}
              className={`nav-item${activeSection === section.id ? " active" : ""}`}
              onClick={() => onNavigate(section.id)}
              aria-current={activeSection === section.id ? "page" : undefined}
              title={section.title}
            >
              <span className="nav-item__icon">{section.icon}</span>
              <span className="nav-item__label">{section.title}</span>
            </button>
          ))}
        </nav>

        <div className="sidebar-footer--compact">
          <div className="sidebar-footer__status-dot" title={`运行状态：${trayStatus}`}>
            <span
              className={`status-indicator status-indicator--${serviceStateConfig[deriveLauncherPresentation(snapshot).state].status}`}
              aria-label={`运行状态：${trayStatus}`}
            />
          </div>
          <button
            type="button"
            className="sidebar-icon-btn"
            onClick={toggleMode}
            title={`当前主题：${modeLabels[mode]}，点击切换`}
            aria-label={`当前主题：${modeLabels[mode]}，点击切换`}
          >
            {modeIcons[mode]}
          </button>
          <button
            type="button"
            className="sidebar-icon-btn"
            onClick={onRefresh}
            title="刷新状态"
            aria-label="刷新启动器状态"
          >
            <ArrowClockwise20Regular />
          </button>
        </div>
      </aside>
    </>
  );
}

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { FluentProvider } from "@fluentui/react-components";
import { App } from "./App";
import { LauncherErrorBoundary } from "./LauncherErrorBoundary";
import { LauncherMotionConfig } from "./LauncherMotionConfig";
import { ThemeProvider, useTheme } from "./useTheme";
import { launcherFluentThemes } from "./launcherTheme";
import { installTrustedNavigationGuards } from "./trustedNavigation";
import { installWailsDesktopApi } from "./wailsDesktopApi";
import { startGlassSurfaces } from "./glassSurfaces";
import "./style.css";
import "./liquid-glass.css";

const uninstallTrustedNavigationGuards = installTrustedNavigationGuards();
const uninstallWailsDesktopApi = installWailsDesktopApi();
const stopGlassSurfaces = startGlassSurfaces(document.body);

function ThemedApp() {
  const { effectiveTheme } = useTheme();
  const theme = launcherFluentThemes[effectiveTheme];

  return (
    <LauncherMotionConfig>
      <FluentProvider theme={theme} className="launcher-fluent-provider">
        <div className="launcher-theme">
          <LauncherErrorBoundary>
            <App />
          </LauncherErrorBoundary>
        </div>
      </FluentProvider>
    </LauncherMotionConfig>
  );
}

const root = createRoot(document.getElementById("app")!);
root.render(
  <StrictMode>
    <ThemeProvider>
      <ThemedApp />
    </ThemeProvider>
  </StrictMode>,
);

if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    root.unmount();
    stopGlassSurfaces();
    uninstallWailsDesktopApi();
    uninstallTrustedNavigationGuards();
  });
}

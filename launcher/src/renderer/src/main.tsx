import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { FluentProvider } from "@fluentui/react-components";
import { App } from "./App";
import { LauncherErrorBoundary } from "./LauncherErrorBoundary";
import { ThemeProvider, useTheme } from "./useTheme";
import { launcherFluentThemes } from "./launcherTheme";
import "./style.css";

function ThemedApp() {
  const { effectiveTheme } = useTheme();
  const theme = launcherFluentThemes[effectiveTheme];

  return (
    <FluentProvider theme={theme} className="launcher-fluent-provider">
      <div className="launcher-theme">
        <LauncherErrorBoundary>
          <App />
        </LauncherErrorBoundary>
      </div>
    </FluentProvider>
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

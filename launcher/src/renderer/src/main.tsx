import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { FluentProvider, webDarkTheme, webLightTheme } from "@fluentui/react-components";
import { App } from "./App";
import { LauncherErrorBoundary } from "./LauncherErrorBoundary";
import { ThemeProvider, useTheme } from "./useTheme";
import "./style.css";

const brandTokens = {
  colorBrandBackground: "#1677ff",
  colorBrandBackground2: "rgba(22, 119, 255, 0.15)",
  colorBrandBackgroundHover: "#4096ff",
  colorBrandBackground2Hover: "rgba(22, 119, 255, 0.22)",
  colorBrandBackgroundPressed: "#0958d9",
  colorBrandBackground2Pressed: "rgba(22, 119, 255, 0.10)",
  colorBrandForeground1: "#1677ff",
  colorBrandForeground2: "#4096ff",
  colorBrandForegroundLink: "#1677ff",
  colorBrandStroke1: "#1677ff",
  colorBrandStroke2: "rgba(22, 119, 255, 0.30)",
  colorCompoundBrandBackground: "#1677ff",
  colorCompoundBrandBackgroundHover: "#4096ff",
  colorCompoundBrandBackgroundPressed: "#0958d9",
  colorCompoundBrandStroke: "rgba(22, 119, 255, 0.64)",
  colorCompoundBrandStrokeHover: "rgba(22, 119, 255, 0.82)",
  colorCompoundBrandStrokePressed: "rgba(22, 119, 255, 0.78)",
  colorNeutralForegroundOnBrand: "#ffffff",
  colorStrokeFocus2: "#1677ff",
};

const darkTheme = { ...webDarkTheme, ...brandTokens };
const lightTheme = { ...webLightTheme, ...brandTokens };

function ThemedApp() {
  const { effectiveTheme } = useTheme();
  const theme = effectiveTheme === "light" ? lightTheme : darkTheme;

  return (
    <FluentProvider
      theme={theme}
      className="launcher-theme"
      style={{ background: "transparent", minHeight: "100vh" }}
    >
      <LauncherErrorBoundary>
        <App />
      </LauncherErrorBoundary>
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

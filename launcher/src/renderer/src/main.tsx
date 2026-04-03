import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { FluentProvider, webDarkTheme } from "@fluentui/react-components";
import { App } from "./App";
import { LauncherErrorBoundary } from "./LauncherErrorBoundary";
import "./style.css";

const launcherTheme = {
  ...webDarkTheme,
  colorBrandBackground: "#7dd8ff",
  colorBrandBackground2: "#17334a",
  colorBrandBackgroundHover: "#98e3ff",
  colorBrandBackground2Hover: "#20425d",
  colorBrandBackgroundPressed: "#68c9f5",
  colorBrandBackground2Pressed: "#112c3f",
  colorBrandForeground1: "#e7faff",
  colorBrandForeground2: "#a7eaff",
  colorBrandForegroundLink: "#bdeeff",
  colorBrandStroke1: "#7dd8ff",
  colorBrandStroke2: "#345b75",
  colorCompoundBrandBackground: "#7dd8ff",
  colorCompoundBrandBackgroundHover: "#98e3ff",
  colorCompoundBrandBackgroundPressed: "#68c9f5",
  colorCompoundBrandStroke: "rgba(125, 216, 255, 0.64)",
  colorCompoundBrandStrokeHover: "rgba(152, 227, 255, 0.82)",
  colorCompoundBrandStrokePressed: "rgba(104, 201, 245, 0.78)",
  colorNeutralForegroundOnBrand: "#05111c",
  colorStrokeFocus2: "#9ce6ff",
};

const root = createRoot(document.getElementById("app")!);
root.render(
  <StrictMode>
    <FluentProvider
      theme={launcherTheme}
      className="launcher-theme"
      style={{ background: "transparent", minHeight: "100vh" }}
    >
      <LauncherErrorBoundary>
        <App />
      </LauncherErrorBoundary>
    </FluentProvider>
  </StrictMode>,
);

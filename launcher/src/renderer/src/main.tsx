import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { FluentProvider, webDarkTheme } from "@fluentui/react-components";
import { App } from "./App";
import "./style.css";

const root = createRoot(document.getElementById("app")!);
root.render(
  <StrictMode>
    <FluentProvider theme={webDarkTheme}>
      <App />
    </FluentProvider>
  </StrictMode>,
);

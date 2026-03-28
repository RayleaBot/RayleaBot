/// <reference types="vite/client" />

import type { LauncherDesktopApi } from "../../shared/desktop-api";

declare global {
  interface Window {
    rayleaLauncher: LauncherDesktopApi;
  }
}

export {};

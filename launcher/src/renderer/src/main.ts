import { createApp } from "vue";
import { System } from "@wailsio/runtime";

import LauncherRoot from "./LauncherRoot.vue";
import { installTrustedNavigationGuards } from "./trustedNavigation";
import { installWailsDesktopApi } from "./wailsDesktopApi";
import "./style.css";
import "./surfaces.css";

const uninstallTrustedNavigationGuards = installTrustedNavigationGuards();
const uninstallWailsDesktopApi = installWailsDesktopApi();

document.documentElement.dataset.windowFrame = System.IsLinux() ? "client" : "native";

const app = createApp(LauncherRoot);
app.mount("#app");

if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    app.unmount();
    uninstallWailsDesktopApi();
    uninstallTrustedNavigationGuards();
  });
}

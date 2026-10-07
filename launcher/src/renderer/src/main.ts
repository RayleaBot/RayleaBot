import { createApp } from "vue";

import LauncherRoot from "./LauncherRoot.vue";
import { installTrustedNavigationGuards } from "./trustedNavigation";
import { installWailsDesktopApi } from "./wailsDesktopApi";
import "./style.css";
import "./surfaces.css";

const uninstallTrustedNavigationGuards = installTrustedNavigationGuards();
const uninstallWailsDesktopApi = installWailsDesktopApi();

const app = createApp(LauncherRoot);
app.mount("#app");

if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    app.unmount();
    uninstallWailsDesktopApi();
    uninstallTrustedNavigationGuards();
  });
}

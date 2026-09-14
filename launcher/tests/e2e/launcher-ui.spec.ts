import { expect, test, type Page } from "@playwright/test";
import path from "node:path";

const fixtureModule = `/@fs/${path.resolve(import.meta.dirname, "../helpers/snapshot.ts").split(path.sep).join("/")}`;

async function mockDesktop(page: Page, options: { failInitialize?: boolean } = {}) {
  // The renderer and its fixture schema are real; desktop effects stay inside
  // this test adapter so a browser run never starts or stops a user's service.
  const overrides = {
    launcher: {
      settings: { installationRoot: "C:\\RayleaBot", closeBehavior: "ask_every_time" },
      resolvedSettings: { installationRoot: "C:\\RayleaBot", serverExecutablePath: "C:\\RayleaBot\\server\\raylea-server.exe", configPath: "C:\\RayleaBot\\config\\user.yaml", workdir: "C:\\RayleaBot" },
      preflightChecks: [{ scope: "preflight", code: "server.executable", title: "服务程序", severity: "ok", summary: "已找到服务程序", detail: "", remediation: "" }],
      runtimePrepare: null,
    },
  };
  await page.route("**/src/wailsDesktopApi.ts*", route => route.fulfill({
    contentType: "application/javascript",
    body: `
      import { createLauncherSnapshot } from ${JSON.stringify(fixtureModule)};
      export function installWailsDesktopApi() {
        const snapshot = createLauncherSnapshot(${JSON.stringify(overrides)});
        window.rayleaLauncher = new Proxy({
          initialize: async () => { if (${Boolean(options.failInitialize)}) throw new Error("fixture initialization failed"); },
          getSnapshot: async () => snapshot,
          getPlatform: async () => "win32-x64",
          isMaximized: async () => false,
          hasPendingCloseConfirm: async () => false,
          hasPendingExternalStopConfirm: async () => false,
          previewResolvedSettings: async () => snapshot.launcher.resolvedSettings,
        }, { get(target, key) {
          if (key in target) return target[key];
          if (String(key).startsWith("on")) return () => () => {};
          return async () => {};
        }});
        return () => {};
      }
    `,
  }));
}

test("initialization failure cannot turn an empty environment result into success", async ({ page }) => {
  await mockDesktop(page, { failInitialize: true });
  await page.goto("/");
  await page.getByRole("button", { name: "环境检查", exact: true }).click();
  await expect(page.getByRole("heading", { name: "检查结果不可用" })).toBeVisible();
  await expect(page.getByText("可以启动", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "重新检查" })).toBeEnabled();
});

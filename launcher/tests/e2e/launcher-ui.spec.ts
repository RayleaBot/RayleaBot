import { expect, test, type Page } from "@playwright/test";
import path from "node:path";

const fixtureModule = `/@fs/${path.resolve(import.meta.dirname, "../helpers/snapshot.ts").split(path.sep).join("/")}`;

async function mockDesktop(page: Page, options: { failInitialize?: boolean; runtimePrepare?: object } = {}) {
  // The renderer and its fixture schema are real; desktop effects stay inside
  // this test adapter so a browser run never starts or stops a user's service.
  const overrides = {
    launcher: {
      settings: { installationRoot: "C:\\RayleaBot", closeBehavior: "ask_every_time" },
      resolvedSettings: { installationRoot: "C:\\RayleaBot", serverExecutablePath: "C:\\RayleaBot\\server\\raylea-server.exe", configPath: "C:\\RayleaBot\\config\\user.yaml", workdir: "C:\\RayleaBot" },
      preflightChecks: [{ scope: "preflight", code: "server.executable", title: "服务程序", severity: "ok", summary: "已找到服务程序", detail: "", remediation: "" }],
      runtimePrepare: options.runtimePrepare ?? null,
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

test("navigation and theme controls stay below the titlebar at supported window sizes", async ({ page }) => {
  await mockDesktop(page);
  await page.goto("/");
  for (const viewport of [{ width: 1280, height: 720 }, { width: 899, height: 650 }, { width: 760, height: 560 }]) {
    await page.setViewportSize(viewport);
    await expect(page.getByRole("heading", { name: "运行状态", exact: true })).toBeVisible();
    const titlebar = await page.locator(".window-drag-handle").boundingBox();
    const theme = await page.getByRole("button", { name: "主题：跟随系统", exact: true }).boundingBox();
    expect(theme!.y).toBeGreaterThanOrEqual(titlebar!.y + titlebar!.height);
    if (viewport.width < 900) {
      expect(theme!.width).toBeGreaterThanOrEqual(44);
      expect(theme!.height).toBeGreaterThanOrEqual(44);
    }
    for (const label of ["环境检查", "日志诊断", "偏好设置", "关于应用", "运行状态"]) {
      await page.getByRole("button", { name: label, exact: true }).click();
      await expect(page.getByRole("heading", { name: label, exact: true })).toBeVisible();
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  }
});

test("development loads the declared heading font without denied asset requests", async ({ page }) => {
  await mockDesktop(page);
  const fontResponses: number[] = [];
  page.on("response", response => { if (response.url().includes(".woff2")) fontResponses.push(response.status()); });
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "运行状态", exact: true })).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
  expect(fontResponses.length).toBeGreaterThan(0);
  expect(fontResponses.every(status => status === 200)).toBe(true);
  expect(await page.evaluate(() => [...document.fonts].some(font => font.family.includes("Noto Sans SC") && font.status === "loaded"))).toBe(true);
});

test("initialization failure cannot turn an empty environment result into success", async ({ page }) => {
  await mockDesktop(page, { failInitialize: true });
  await page.goto("/");
  await page.getByRole("button", { name: "环境检查", exact: true }).click();
  await expect(page.getByRole("heading", { name: "检查结果不可用" })).toBeVisible();
  await expect(page.getByText("可以启动", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "重新检查" })).toBeEnabled();
});

test("reduced motion keeps indeterminate progress meaningful and theme selection operable", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await mockDesktop(page, { runtimePrepare: {
    active: true, currentKind: "chromium", summary: "正在准备运行环境。",
    resources: [{ kind: "chromium", label: "Chromium", resourceId: "fixture", version: "1", status: "running", stage: "download", progress: null, downloadedBytes: 1024, totalBytes: null, extractedEntries: null, totalEntries: null, sourceUrl: "", sourceLabel: "", archivePath: "", storeRoot: "", error: "", updatedAt: "2026-09-08T00:00:00Z" }],
  } });
  await page.goto("/");
  const progress = page.getByRole("progressbar", { name: "Chromium准备进度" });
  await expect(progress).toBeVisible();
  await expect(progress).not.toHaveAttribute("aria-valuenow");
  await expect(progress).toHaveAttribute("aria-valuetext", /进行中/);
  const trigger = page.getByRole("button", { name: "主题：跟随系统", exact: true });
  await trigger.click();
  await page.getByRole("menuitemradio", { name: "深色", exact: true }).click();
  await expect(page.getByRole("menu")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "主题：深色", exact: true })).toBeFocused();
});

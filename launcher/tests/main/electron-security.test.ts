import path from "node:path";
import { describe, expect, test, vi } from "vitest";
import {
  LAUNCHER_RENDERER_ORIGIN,
  LAUNCHER_RENDERER_SCHEME,
  LAUNCHER_RENDERER_URL,
  assertTrustedIpcEvent,
  createSecureIpcRegistrar,
  denyRendererPermissions,
  installRendererNavigationGuards,
  launcherRendererContentSecurityPolicy,
  resolveLauncherRendererTarget,
  resolvePackagedRendererAssetPath,
  wirePackagedRendererProtocol,
} from "@main/services/electron-security";

describe("Electron renderer security", () => {
  test("uses the fixed secure protocol and rejects packaged dev-server overrides", () => {
    expect(resolveLauncherRendererTarget({ isPackaged: true, devServerUrl: undefined })).toEqual({
      kind: "packaged",
      origin: LAUNCHER_RENDERER_ORIGIN,
      url: LAUNCHER_RENDERER_URL,
    });
    expect(() =>
      resolveLauncherRendererTarget({
        isPackaged: true,
        devServerUrl: "http://127.0.0.1:5174",
      }),
    ).toThrow("打包版 Launcher 禁止使用 RAYLEA_DEV_SERVER_URL");
  });

  test("allows only a loopback HTTP root as a development renderer", () => {
    expect(
      resolveLauncherRendererTarget({
        isPackaged: false,
        devServerUrl: "http://127.0.0.1:5174/",
      }),
    ).toEqual({
      kind: "development",
      origin: "http://127.0.0.1:5174",
      url: "http://127.0.0.1:5174/",
    });

    for (const url of [
      "https://127.0.0.1:5174/",
      "http://evil.example/",
      "http://user:password@127.0.0.1:5174/",
      "http://127.0.0.1:5174/untrusted",
    ]) {
      expect(() => resolveLauncherRendererTarget({ isPackaged: false, devServerUrl: url })).toThrow(
        "RAYLEA_DEV_SERVER_URL 必须是本机 HTTP 根地址",
      );
    }
  });

  test("maps only app-host assets inside the renderer root", () => {
    const root = path.resolve("src/renderer");
    expect(resolvePackagedRendererAssetPath(root, LAUNCHER_RENDERER_URL)).toBe(
      path.join(root, "index.html"),
    );
    expect(
      resolvePackagedRendererAssetPath(root, `${LAUNCHER_RENDERER_URL}%2e%2e%2fpackage.json`),
    ).toBeNull();
    expect(
      resolvePackagedRendererAssetPath(root, `${LAUNCHER_RENDERER_SCHEME}://other/index.html`),
    ).toBeNull();
  });

  test("serves packaged assets with a strict CSP and rejects non-read requests", async () => {
    let handler: ((request: { method: string; url: string }) => Promise<Response> | Response) | null = null;
    wirePackagedRendererProtocol({
      protocol: {
        handle: (_scheme, nextHandler) => {
          handler = nextHandler;
        },
      },
      rendererRoot: path.resolve("src/renderer"),
    });

    expect(handler).not.toBeNull();
    const getResponse = await handler!({ method: "GET", url: LAUNCHER_RENDERER_URL });
    expect(getResponse.status).toBe(200);
    expect(getResponse.headers.get("content-security-policy")).toContain("script-src 'self'");
    expect(getResponse.headers.get("content-security-policy")).not.toContain("unsafe-eval");
    expect(getResponse.headers.get("x-content-type-options")).toBe("nosniff");

    const postResponse = await handler!({ method: "POST", url: LAUNCHER_RENDERER_URL });
    expect(postResponse.status).toBe(405);
  });

  test("rejects remote renderer frames and subframe IPC", () => {
    const mainFrame = { url: LAUNCHER_RENDERER_URL };
    const mainWebContents = { mainFrame };

    expect(() =>
      assertTrustedIpcEvent({
        event: { sender: mainWebContents, senderFrame: mainFrame },
        expectedOrigin: LAUNCHER_RENDERER_ORIGIN,
        mainWebContents,
      }),
    ).not.toThrow();

    const remoteFrame = { url: "https://evil.example/" };
    expect(() =>
      assertTrustedIpcEvent({
        event: { sender: { mainFrame: remoteFrame }, senderFrame: remoteFrame },
        expectedOrigin: LAUNCHER_RENDERER_ORIGIN,
        mainWebContents,
      }),
    ).toThrow("Launcher IPC 请求来源无效");

    expect(() =>
      assertTrustedIpcEvent({
        event: { sender: mainWebContents, senderFrame: { url: LAUNCHER_RENDERER_URL } },
        expectedOrigin: LAUNCHER_RENDERER_ORIGIN,
        mainWebContents,
      }),
    ).toThrow("Launcher IPC 请求来源无效");
  });

  test("checks IPC argument count before parsing payloads", async () => {
    const listeners = new Map<string, (event: never, ...args: unknown[]) => unknown>();
    const mainFrame = { url: LAUNCHER_RENDERER_URL };
    const mainWebContents = { mainFrame };
    const registrar = createSecureIpcRegistrar({
      ipcMain: {
        handle: (channel, listener) => listeners.set(channel, listener as never),
      },
      expectedOrigin: LAUNCHER_RENDERER_ORIGIN,
      getMainWebContents: () => mainWebContents,
    });
    const event = { sender: mainWebContents, senderFrame: mainFrame } as never;
    const noArgsHandler = vi.fn(() => "ok");
    const oneArgHandler = vi.fn((value: string) => value.toUpperCase());

    registrar.noArgs("no-args", noArgsHandler);
    registrar.oneArg(
      "one-arg",
      (value) => {
        if (typeof value !== "string") throw new Error("invalid payload");
        return value;
      },
      oneArgHandler,
    );

    expect(listeners.get("no-args")!(event)).toBe("ok");
    expect(() => listeners.get("no-args")!(event, "unexpected")).toThrow("Launcher IPC 请求格式无效");
    expect(listeners.get("one-arg")!(event, "safe")).toBe("SAFE");
    expect(() => listeners.get("one-arg")!(event, { unknown: true })).toThrow("invalid payload");
    expect(() => listeners.get("one-arg")!(event)).toThrow("Launcher IPC 请求格式无效");
  });

  test("denies navigation, popup, webview, and permission requests", () => {
    const listeners = new Map<string, (event: { preventDefault(): void }) => void>();
    let windowOpenHandler: (() => { action: "deny" }) | null = null;
    installRendererNavigationGuards({
      on: (event, listener) => listeners.set(event, listener as never),
      setWindowOpenHandler: (handler) => {
        windowOpenHandler = handler;
      },
    });

    for (const event of ["will-navigate", "will-frame-navigate", "will-redirect", "will-attach-webview"]) {
      const preventDefault = vi.fn();
      listeners.get(event)!({ preventDefault });
      expect(preventDefault).toHaveBeenCalledTimes(1);
    }
    expect(windowOpenHandler!()).toEqual({ action: "deny" });

    let permissionCheckHandler: (() => boolean) | null = null;
    let permissionRequestHandler:
      | ((webContents: unknown, permission: unknown, callback: (allowed: boolean) => void) => void)
      | null = null;
    denyRendererPermissions({
      setPermissionCheckHandler: (handler) => {
        permissionCheckHandler = handler;
      },
      setPermissionRequestHandler: (handler) => {
        permissionRequestHandler = handler;
      },
    });
    expect(permissionCheckHandler!()).toBe(false);
    const permissionCallback = vi.fn();
    permissionRequestHandler!(null, "notifications", permissionCallback);
    expect(permissionCallback).toHaveBeenCalledWith(false);
  });

  test("keeps the script policy self-only", () => {
    const policy = launcherRendererContentSecurityPolicy({
      kind: "packaged",
      origin: LAUNCHER_RENDERER_ORIGIN,
      url: LAUNCHER_RENDERER_URL,
    });
    expect(policy).toContain("script-src 'self'");
    expect(policy).not.toContain("script-src 'self' 'unsafe-inline'");
  });
});

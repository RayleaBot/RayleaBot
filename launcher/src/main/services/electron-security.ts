import { readFile } from "node:fs/promises";
import path from "node:path";

export const LAUNCHER_RENDERER_SCHEME = "raylea-launcher";
export const LAUNCHER_RENDERER_HOST = "app";
export const LAUNCHER_RENDERER_URL = `${LAUNCHER_RENDERER_SCHEME}://${LAUNCHER_RENDERER_HOST}/`;
export const LAUNCHER_RENDERER_ORIGIN = `${LAUNCHER_RENDERER_SCHEME}://${LAUNCHER_RENDERER_HOST}`;

const LOOPBACK_HOSTS = new Set(["127.0.0.1", "::1", "localhost"]);
const IPC_SOURCE_ERROR = "Launcher IPC 请求来源无效。";
const IPC_PAYLOAD_ERROR = "Launcher IPC 请求格式无效。";

const CONTENT_TYPES = new Map([
  [".css", "text/css; charset=utf-8"],
  [".gif", "image/gif"],
  [".html", "text/html; charset=utf-8"],
  [".ico", "image/x-icon"],
  [".jpeg", "image/jpeg"],
  [".jpg", "image/jpeg"],
  [".js", "text/javascript; charset=utf-8"],
  [".json", "application/json; charset=utf-8"],
  [".png", "image/png"],
  [".svg", "image/svg+xml"],
  [".webp", "image/webp"],
  [".woff", "font/woff"],
  [".woff2", "font/woff2"],
]);

interface RendererFrameLike {
  readonly url: string;
}

interface RendererWebContentsLike {
  readonly mainFrame: RendererFrameLike;
}

interface IpcInvokeEventLike {
  readonly sender: RendererWebContentsLike;
  readonly senderFrame: RendererFrameLike | null;
}

interface IpcMainLike {
  handle(
    channel: string,
    listener: (event: IpcInvokeEventLike, ...args: unknown[]) => unknown,
  ): void;
}

interface ProtocolRequestLike {
  readonly method: string;
  readonly url: string;
}

interface ProtocolLike {
  handle(
    scheme: string,
    handler: (request: ProtocolRequestLike) => Response | Promise<Response>,
  ): void;
}

interface PreventableEventLike {
  preventDefault(): void;
}

interface RendererWebContentsSecurityLike {
  on(event: "will-navigate" | "will-redirect", listener: (event: PreventableEventLike) => void): unknown;
  on(event: "will-frame-navigate", listener: (event: PreventableEventLike) => void): unknown;
  on(
    event: "will-attach-webview",
    listener: (event: PreventableEventLike, webPreferences: unknown, params: unknown) => void,
  ): unknown;
  setWindowOpenHandler(handler: () => { action: "deny" }): void;
}

interface RendererSessionSecurityLike {
  setPermissionCheckHandler(handler: () => boolean): void;
  setPermissionRequestHandler(
    handler: (
      webContents: unknown,
      permission: unknown,
      callback: (permissionGranted: boolean) => void,
      details: unknown,
    ) => void,
  ): void;
}

export interface LauncherRendererTarget {
  kind: "development" | "packaged";
  url: string;
  origin: string;
}

export function resolveLauncherRendererTarget(input: {
  isPackaged: boolean;
  devServerUrl: string | undefined;
}): LauncherRendererTarget {
  if (input.isPackaged && input.devServerUrl !== undefined) {
    throw new Error("打包版 Launcher 禁止使用 RAYLEA_DEV_SERVER_URL。");
  }

  if (input.devServerUrl === undefined) {
    return {
      kind: "packaged",
      url: LAUNCHER_RENDERER_URL,
      origin: LAUNCHER_RENDERER_ORIGIN,
    };
  }

  let parsed: URL;
  try {
    parsed = new URL(input.devServerUrl);
  } catch {
    throw new Error("RAYLEA_DEV_SERVER_URL 必须是本机 HTTP 地址。");
  }

  if (
    parsed.protocol !== "http:" ||
    !LOOPBACK_HOSTS.has(parsed.hostname) ||
    parsed.username ||
    parsed.password ||
    parsed.pathname !== "/" ||
    parsed.search ||
    parsed.hash
  ) {
    throw new Error("RAYLEA_DEV_SERVER_URL 必须是本机 HTTP 根地址。");
  }

  return {
    kind: "development",
    url: parsed.toString(),
    origin: parsed.origin,
  };
}

export function launcherRendererContentSecurityPolicy(target: LauncherRendererTarget) {
  const developmentConnectSource =
    target.kind === "development" ? ` ${target.origin} ${target.origin.replace(/^http:/, "ws:")}` : "";
  const developmentInlineScript = target.kind === "development" ? " 'unsafe-inline'" : "";

  return [
    "default-src 'none'",
    "base-uri 'none'",
    "child-src 'none'",
    `connect-src 'self'${developmentConnectSource}`,
    "font-src 'self'",
    "form-action 'none'",
    "frame-ancestors 'none'",
    "frame-src 'none'",
    "img-src 'self' data:",
    "media-src 'none'",
    "object-src 'none'",
    `script-src 'self'${developmentInlineScript}`,
    "style-src 'self' 'unsafe-inline'",
    "worker-src 'none'",
  ].join("; ");
}

function securityHeaders(target: LauncherRendererTarget) {
  return {
    "Content-Security-Policy": launcherRendererContentSecurityPolicy(target),
    "Cross-Origin-Opener-Policy": "same-origin",
    "Permissions-Policy": "accelerometer=(), camera=(), geolocation=(), microphone=(), payment=(), usb=()",
    "Referrer-Policy": "no-referrer",
    "X-Content-Type-Options": "nosniff",
  };
}

export function resolvePackagedRendererAssetPath(rendererRoot: string, requestUrl: string) {
  let request: URL;
  try {
    request = new URL(requestUrl);
  } catch {
    return null;
  }

  if (
    request.protocol !== `${LAUNCHER_RENDERER_SCHEME}:` ||
    request.hostname !== LAUNCHER_RENDERER_HOST ||
    request.port ||
    request.username ||
    request.password
  ) {
    return null;
  }

  let decodedPath: string;
  try {
    decodedPath = decodeURIComponent(request.pathname);
  } catch {
    return null;
  }

  const relativePath = decodedPath === "/" ? "index.html" : decodedPath.replace(/^\/+/, "");
  const segments = relativePath.split("/");
  if (
    !relativePath ||
    relativePath.includes("\\") ||
    relativePath.includes("\0") ||
    segments.some((segment) => !segment || segment === "." || segment === "..")
  ) {
    return null;
  }

  const root = path.resolve(rendererRoot);
  const resolved = path.resolve(root, ...segments);
  const relative = path.relative(root, resolved);
  if (!relative || relative.startsWith("..") || path.isAbsolute(relative)) {
    return null;
  }
  return resolved;
}

export function wirePackagedRendererProtocol(input: {
  protocol: ProtocolLike;
  rendererRoot: string;
}) {
  const target: LauncherRendererTarget = {
    kind: "packaged",
    url: LAUNCHER_RENDERER_URL,
    origin: LAUNCHER_RENDERER_ORIGIN,
  };

  input.protocol.handle(LAUNCHER_RENDERER_SCHEME, async (request) => {
    if (request.method !== "GET" && request.method !== "HEAD") {
      return new Response(null, { status: 405, headers: securityHeaders(target) });
    }

    const assetPath = resolvePackagedRendererAssetPath(input.rendererRoot, request.url);
    if (!assetPath) {
      return new Response(null, { status: 404, headers: securityHeaders(target) });
    }

    try {
      const content = request.method === "HEAD" ? null : new Uint8Array(await readFile(assetPath));
      return new Response(content, {
        status: 200,
        headers: {
          ...securityHeaders(target),
          "Cache-Control": path.extname(assetPath) === ".html" ? "no-store" : "no-cache",
          "Content-Type": CONTENT_TYPES.get(path.extname(assetPath).toLowerCase()) ?? "application/octet-stream",
        },
      });
    } catch {
      return new Response(null, { status: 404, headers: securityHeaders(target) });
    }
  });
}

export function installRendererNavigationGuards(webContents: RendererWebContentsSecurityLike) {
  const deny = (event: PreventableEventLike) => event.preventDefault();
  webContents.on("will-navigate", deny);
  webContents.on("will-frame-navigate", deny);
  webContents.on("will-redirect", deny);
  webContents.on("will-attach-webview", deny);
  webContents.setWindowOpenHandler(() => ({ action: "deny" }));
}

export function denyRendererPermissions(session: RendererSessionSecurityLike) {
  session.setPermissionCheckHandler(() => false);
  session.setPermissionRequestHandler((_webContents, _permission, callback) => callback(false));
}

function rendererOriginFromUrl(rawUrl: string) {
  let parsed: URL;
  try {
    parsed = new URL(rawUrl);
  } catch {
    return null;
  }

  if (
    parsed.protocol === `${LAUNCHER_RENDERER_SCHEME}:` &&
    parsed.hostname === LAUNCHER_RENDERER_HOST &&
    !parsed.port &&
    !parsed.username &&
    !parsed.password
  ) {
    return LAUNCHER_RENDERER_ORIGIN;
  }
  return parsed.origin === "null" ? null : parsed.origin;
}

export function assertTrustedIpcEvent(input: {
  event: IpcInvokeEventLike;
  expectedOrigin: string;
  mainWebContents: RendererWebContentsLike | null;
}) {
  const { event, expectedOrigin, mainWebContents } = input;
  if (
    !mainWebContents ||
    event.sender !== mainWebContents ||
    !event.senderFrame ||
    event.senderFrame !== event.sender.mainFrame ||
    rendererOriginFromUrl(event.senderFrame.url) !== expectedOrigin
  ) {
    throw new Error(IPC_SOURCE_ERROR);
  }
}

export function createSecureIpcRegistrar(input: {
  ipcMain: IpcMainLike;
  expectedOrigin: string;
  getMainWebContents: () => RendererWebContentsLike | null;
}) {
  function validateEvent(event: IpcInvokeEventLike) {
    assertTrustedIpcEvent({
      event,
      expectedOrigin: input.expectedOrigin,
      mainWebContents: input.getMainWebContents(),
    });
  }

  return {
    noArgs(channel: string, handler: () => unknown) {
      input.ipcMain.handle(channel, (event, ...args) => {
        validateEvent(event);
        if (args.length !== 0) {
          throw new Error(IPC_PAYLOAD_ERROR);
        }
        return handler();
      });
    },
    oneArg<T>(channel: string, parse: (value: unknown) => T, handler: (value: T) => unknown) {
      input.ipcMain.handle(channel, (event, ...args) => {
        validateEvent(event);
        if (args.length !== 1) {
          throw new Error(IPC_PAYLOAD_ERROR);
        }
        return handler(parse(args[0]));
      });
    },
  };
}

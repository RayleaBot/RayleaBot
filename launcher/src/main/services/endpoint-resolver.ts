import fs from "node:fs/promises";
import YAML from "yaml";
import type { ServerEndpoint } from "../../shared/launcher-models";

export interface ResolveServerEndpointWarning {
  configPath: string;
  message: string;
  cause?: unknown;
}

interface ResolveServerEndpointOptions {
  onWarning?: (warning: ResolveServerEndpointWarning) => void;
}

function normalizeClientHost(host: string) {
  const trimmed = host.trim().replace(/^\[/, "").replace(/\]$/, "");
  if (!trimmed || trimmed === "0.0.0.0" || trimmed === "::" || trimmed === "*") {
    return "127.0.0.1";
  }
  return trimmed;
}

function formatBaseUrlHost(host: string) {
  return host.includes(":") ? `[${host}]` : host;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function emitResolverWarning(
  configPath: string,
  error: unknown,
  options: ResolveServerEndpointOptions,
) {
  const message = `无法从 ${configPath} 读取服务监听配置，已回退到 127.0.0.1:8080。`;
  if (options.onWarning) {
    options.onWarning({ configPath, message, cause: error });
    return;
  }

  console.warn(message, error);
}

export async function resolveServerEndpoint(
  configPath: string,
  options: ResolveServerEndpointOptions = {},
): Promise<ServerEndpoint> {
  let host = "127.0.0.1";
  let port = 8080;

  try {
    const text = await fs.readFile(configPath, "utf8");
    const payload = YAML.parse(text) as unknown;
    const server = isRecord(payload) ? payload.server : undefined;

    if (isRecord(server)) {
      const rawHost = server.host;
      if (typeof rawHost === "string") {
        host = normalizeClientHost(rawHost);
      }

      const rawPort = server.port;
      if (typeof rawPort === "number" && Number.isInteger(rawPort)) {
        port = rawPort;
      } else if (typeof rawPort === "string") {
        const parsed = Number.parseInt(rawPort.trim(), 10);
        if (!Number.isNaN(parsed)) {
          port = parsed;
        }
      }
    }
  } catch (error) {
    emitResolverWarning(configPath, error, options);
  }

  return {
    host,
    port,
    baseUrl: `http://${formatBaseUrlHost(host)}:${port}/`,
  };
}

import fs from "node:fs/promises";
import type { ServerEndpoint } from "../../shared/launcher-models";

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

export async function resolveServerEndpoint(configPath: string): Promise<ServerEndpoint> {
  let host = "127.0.0.1";
  let port = 8080;

  try {
    const text = await fs.readFile(configPath, "utf8");
    let insideServer = false;
    for (const rawLine of text.split(/\r?\n/)) {
      const withoutComment = rawLine.split("#", 1)[0]?.trimEnd() ?? "";
      if (!withoutComment.trim()) {
        continue;
      }
      if (!/^\s/.test(rawLine)) {
        insideServer = withoutComment.trim() === "server:";
        continue;
      }
      if (!insideServer) {
        continue;
      }
      const trimmed = withoutComment.trim();
      if (trimmed.startsWith("host:")) {
        host = normalizeClientHost(trimmed.slice("host:".length).trim().replace(/^['"]|['"]$/g, ""));
      }
      if (trimmed.startsWith("port:")) {
        const parsed = Number.parseInt(trimmed.slice("port:".length).trim().replace(/^['"]|['"]$/g, ""), 10);
        if (!Number.isNaN(parsed)) {
          port = parsed;
        }
      }
    }
  } catch {
    // Fall back to default loopback endpoint when the config is missing or unreadable.
  }

  return {
    host,
    port,
    baseUrl: `http://${formatBaseUrlHost(host)}:${port}/`,
  };
}

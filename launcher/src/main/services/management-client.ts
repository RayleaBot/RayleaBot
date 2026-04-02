import type { RecoveryCompatibilitySummary, ServerEndpoint } from "../../shared/launcher-models";

const DEFAULT_REQUEST_TIMEOUT_MS = 5000;

type FetchLike = typeof fetch;

async function readJson(response: Response) {
  return (await response.json()) as Record<string, unknown>;
}

async function ensureSuccess(response: Response) {
  if (response.ok) {
    return response;
  }
  const body = await response.text();
  throw new Error(body || `${response.status} ${response.statusText}`);
}

function createAuthedHeaders(sessionToken: string) {
  return { Authorization: `Bearer ${sessionToken}` };
}

function withTimeout(init: RequestInit | undefined, timeoutMs: number): RequestInit {
  return {
    ...init,
    signal: AbortSignal.timeout(timeoutMs),
  };
}

export class FetchLauncherManagementClient {
  private readonly fetchLike: FetchLike;
  private readonly timeoutMs: number;

  constructor(options: { fetchLike?: FetchLike; timeoutMs?: number } = {}) {
    this.fetchLike = options.fetchLike ?? fetch;
    this.timeoutMs = options.timeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS;
  }

  async isHealthy(endpoint: ServerEndpoint) {
    try {
      const response = await this.fetchLike(new URL("healthz", endpoint.baseUrl), withTimeout(undefined, this.timeoutMs));
      return response.ok;
    } catch {
      return false;
    }
  }

  async getSetupInitialized(endpoint: ServerEndpoint) {
    const response = await ensureSuccess(
      await this.fetchLike(new URL("api/setup/status", endpoint.baseUrl), withTimeout(undefined, this.timeoutMs)),
    );
    const payload = await readJson(response);
    return Boolean(payload.initialized);
  }

  async issueLauncherToken(endpoint: ServerEndpoint) {
    const response = await ensureSuccess(
      await this.fetchLike(
        new URL("api/session/launcher-token", endpoint.baseUrl),
        withTimeout({ method: "POST" }, this.timeoutMs),
      ),
    );
    const payload = await readJson(response);
    return String(payload.launcher_token ?? "");
  }

  async admitLauncherToken(endpoint: ServerEndpoint, launcherToken: string) {
    const response = await ensureSuccess(
      await this.fetchLike(
        new URL("api/session/launcher-admission", endpoint.baseUrl),
        withTimeout(
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ launcher_token: launcherToken }),
          },
          this.timeoutMs,
        ),
      ),
    );
    const payload = await readJson(response);
    return String(payload.session_token ?? "");
  }

  async shutdown(endpoint: ServerEndpoint, sessionToken: string) {
    await ensureSuccess(
      await this.fetchLike(
        new URL("api/system/shutdown", endpoint.baseUrl),
        withTimeout(
          {
            method: "POST",
            headers: createAuthedHeaders(sessionToken),
          },
          this.timeoutMs,
        ),
      ),
    );
  }

  async getSystemStatus(endpoint: ServerEndpoint, sessionToken: string): Promise<{ recovery_summary?: RecoveryCompatibilitySummary | null }> {
    const response = await ensureSuccess(
      await this.fetchLike(
        new URL("api/system/status", endpoint.baseUrl),
        withTimeout(
          {
            headers: createAuthedHeaders(sessionToken),
          },
          this.timeoutMs,
        ),
      ),
    );
    return (await readJson(response)) as { recovery_summary?: RecoveryCompatibilitySummary | null };
  }
}

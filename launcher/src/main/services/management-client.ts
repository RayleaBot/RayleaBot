import type {
  LauncherReadinessSnapshot,
  LauncherSystemStatusSnapshot,
  ServerEndpoint,
} from "../../shared/launcher-models";

const DEFAULT_REQUEST_TIMEOUT_MS = 5000;

type FetchLike = typeof fetch;

async function readJson<T>(response: Response) {
  return (await response.json()) as T;
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

  async getReadiness(endpoint: ServerEndpoint): Promise<LauncherReadinessSnapshot> {
    const response = await this.fetchLike(new URL("readyz", endpoint.baseUrl), withTimeout(undefined, this.timeoutMs));
    if (response.status === 200 || response.status === 503) {
      return await readJson<LauncherReadinessSnapshot>(response);
    }
    await ensureSuccess(response);
    return { status: "failed" };
  }

  async getSetupInitialized(endpoint: ServerEndpoint) {
    const response = await ensureSuccess(
      await this.fetchLike(new URL("api/setup/status", endpoint.baseUrl), withTimeout(undefined, this.timeoutMs)),
    );
    const payload = await readJson<Record<string, unknown>>(response);
    return Boolean(payload.initialized);
  }

  async issueLauncherToken(endpoint: ServerEndpoint) {
    const response = await ensureSuccess(
      await this.fetchLike(
        new URL("api/session/launcher-token", endpoint.baseUrl),
        withTimeout({ method: "POST" }, this.timeoutMs),
      ),
    );
    const payload = await readJson<Record<string, unknown>>(response);
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
    const payload = await readJson<Record<string, unknown>>(response);
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

  async getSystemStatus(endpoint: ServerEndpoint, sessionToken: string): Promise<LauncherSystemStatusSnapshot> {
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
    return await readJson<LauncherSystemStatusSnapshot>(response);
  }

  async createRecoveryRecheck(endpoint: ServerEndpoint, sessionToken: string) {
    const response = await ensureSuccess(
      await this.fetchLike(
        new URL("api/system/recovery/recheck", endpoint.baseUrl),
        withTimeout(
          {
            method: "POST",
            headers: createAuthedHeaders(sessionToken),
          },
          this.timeoutMs,
        ),
      ),
    );
    return await readJson<{ task_id: string }>(response);
  }

  async createRuntimeBootstrap(endpoint: ServerEndpoint, sessionToken: string, resources?: string[]) {
    const response = await ensureSuccess(
      await this.fetchLike(
        new URL("api/system/runtime/bootstrap", endpoint.baseUrl),
        withTimeout(
          {
            method: "POST",
            headers: {
              ...createAuthedHeaders(sessionToken),
              "Content-Type": "application/json",
            },
            body: JSON.stringify(resources?.length ? { resources } : {}),
          },
          this.timeoutMs,
        ),
      ),
    );
    return await readJson<{ task_id: string }>(response);
  }
}

import type { ServerEndpoint } from "../../shared/launcher-models";

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

export class FetchLauncherManagementClient {
  async isHealthy(endpoint: ServerEndpoint) {
    const response = await fetch(new URL("healthz", endpoint.baseUrl));
    return response.ok;
  }

  async getSetupInitialized(endpoint: ServerEndpoint) {
    const response = await ensureSuccess(await fetch(new URL("api/setup/status", endpoint.baseUrl)));
    const payload = await readJson(response);
    return Boolean(payload.initialized);
  }

  async issueLauncherToken(endpoint: ServerEndpoint) {
    const response = await ensureSuccess(
      await fetch(new URL("api/session/launcher-token", endpoint.baseUrl), { method: "POST" }),
    );
    const payload = await readJson(response);
    return String(payload.launcher_token ?? "");
  }

  async admitLauncherToken(endpoint: ServerEndpoint, launcherToken: string) {
    const response = await ensureSuccess(
      await fetch(new URL("api/session/launcher-admission", endpoint.baseUrl), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ launcher_token: launcherToken }),
      }),
    );
    const payload = await readJson(response);
    return String(payload.session_token ?? "");
  }

  async shutdown(endpoint: ServerEndpoint, sessionToken: string) {
    await ensureSuccess(
      await fetch(new URL("api/system/shutdown", endpoint.baseUrl), {
        method: "POST",
        headers: createAuthedHeaders(sessionToken),
      }),
    );
  }
}

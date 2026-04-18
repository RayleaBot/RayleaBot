import type {
  ErrorEnvelope,
  LauncherAdmissionRequest,
  LauncherReadinessSnapshot,
  LauncherSystemStatusSnapshot,
  LauncherTokenResponse,
  ServerEndpoint,
  TaskAcceptedResponse,
  TaskListResponse,
  TaskSummary,
} from "../../shared/launcher-models";

const DEFAULT_REQUEST_TIMEOUT_MS = 5000;
const IN_PROGRESS_TASK_STATUSES = new Set(["pending", "running"]);

type FetchLike = typeof fetch;

class LauncherManagementError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;
  readonly details?: Record<string, unknown>;

  constructor(
    message: string,
    status: number,
    code = "platform.unknown",
    requestId?: string,
    details?: Record<string, unknown>,
  ) {
    super(message);
    this.name = "LauncherManagementError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
    this.details = details;
  }
}

async function readJson<T>(response: Response) {
  return (await response.json()) as T;
}

async function readPayload(response: Response) {
  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    return await response.json();
  }
  return await response.text();
}

function readErrorEnvelope(payload: unknown): ErrorEnvelope | null {
  if (typeof payload !== "object" || payload === null || !("error" in payload)) {
    return null;
  }
  return payload as ErrorEnvelope;
}

function buildErrorMessage(response: Response, payload: unknown) {
  const envelope = readErrorEnvelope(payload);
  if (envelope?.error.message?.trim()) {
    return envelope.error.message.trim();
  }
  if (typeof payload === "string" && payload.trim()) {
    return payload.trim();
  }
  return `${response.status} ${response.statusText}`.trim();
}

function buildResponseError(response: Response, payload: unknown) {
  const envelope = readErrorEnvelope(payload);
  return new LauncherManagementError(
    buildErrorMessage(response, payload),
    response.status,
    envelope?.error.code,
    envelope?.error.request_id,
    envelope?.error.details,
  );
}

async function ensureSuccess(response: Response) {
  if (response.ok) {
    return response;
  }
  throw buildResponseError(response, await readPayload(response));
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

async function fetchWithTimeout(fetchLike: FetchLike, input: URL, init: RequestInit | undefined, timeoutMs: number) {
  try {
    return await fetchLike(input, withTimeout(init, timeoutMs));
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw new LauncherManagementError("请求超时。", 0);
    }
    if (error instanceof Error) {
      throw error;
    }
    throw new LauncherManagementError("请求失败。", 0);
  }
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
      const response = await fetchWithTimeout(this.fetchLike, new URL("healthz", endpoint.baseUrl), undefined, this.timeoutMs);
      return response.ok;
    } catch {
      return false;
    }
  }

  async getReadiness(endpoint: ServerEndpoint): Promise<LauncherReadinessSnapshot> {
    const response = await fetchWithTimeout(this.fetchLike, new URL("readyz", endpoint.baseUrl), undefined, this.timeoutMs);
    if (response.status === 200 || response.status === 503) {
      return await readJson<LauncherReadinessSnapshot>(response);
    }
    await ensureSuccess(response);
    return { status: "failed" };
  }

  async getSetupInitialized(endpoint: ServerEndpoint) {
    const response = await ensureSuccess(
      await fetchWithTimeout(this.fetchLike, new URL("api/setup/status", endpoint.baseUrl), undefined, this.timeoutMs),
    );
    const payload = await readJson<Record<string, unknown>>(response);
    return Boolean(payload.initialized);
  }

  async issueLauncherToken(endpoint: ServerEndpoint) {
    const response = await ensureSuccess(
      await fetchWithTimeout(
        this.fetchLike,
        new URL("api/session/launcher-token", endpoint.baseUrl),
        { method: "POST" },
        this.timeoutMs,
      ),
    );
    const payload = await readJson<LauncherTokenResponse>(response);
    return String(payload.launcher_token ?? "");
  }

  async admitLauncherToken(endpoint: ServerEndpoint, launcherToken: string) {
    const body: LauncherAdmissionRequest = { launcher_token: launcherToken };
    const response = await ensureSuccess(
      await fetchWithTimeout(
        this.fetchLike,
        new URL("api/session/launcher-admission", endpoint.baseUrl),
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        },
        this.timeoutMs,
      ),
    );
    const payload = await readJson<Record<string, unknown>>(response);
    return String(payload.session_token ?? "");
  }

  async shutdown(endpoint: ServerEndpoint, sessionToken: string) {
    await ensureSuccess(
      await fetchWithTimeout(
        this.fetchLike,
        new URL("api/system/shutdown", endpoint.baseUrl),
        {
          method: "POST",
          headers: createAuthedHeaders(sessionToken),
        },
        this.timeoutMs,
      ),
    );
  }

  async getSystemStatus(endpoint: ServerEndpoint, sessionToken: string): Promise<LauncherSystemStatusSnapshot> {
    const response = await ensureSuccess(
      await fetchWithTimeout(
        this.fetchLike,
        new URL("api/system/status", endpoint.baseUrl),
        {
          headers: createAuthedHeaders(sessionToken),
        },
        this.timeoutMs,
      ),
    );
    return await readJson<LauncherSystemStatusSnapshot>(response);
  }

  async findInProgressTask(endpoint: ServerEndpoint, sessionToken: string, taskType: string): Promise<TaskSummary | null> {
    const response = await ensureSuccess(
      await fetchWithTimeout(
        this.fetchLike,
        new URL(`api/tasks?task_type=${encodeURIComponent(taskType)}`, endpoint.baseUrl),
        {
          headers: createAuthedHeaders(sessionToken),
        },
        this.timeoutMs,
      ),
    );
    const payload = await readJson<TaskListResponse>(response);
    return payload.items.find((task) => IN_PROGRESS_TASK_STATUSES.has(task.status)) ?? null;
  }

  async createRecoveryRecheck(endpoint: ServerEndpoint, sessionToken: string) {
    const response = await ensureSuccess(
      await fetchWithTimeout(
        this.fetchLike,
        new URL("api/system/recovery/recheck", endpoint.baseUrl),
        {
          method: "POST",
          headers: createAuthedHeaders(sessionToken),
        },
        this.timeoutMs,
      ),
    );
    return await readJson<TaskAcceptedResponse>(response);
  }

  async createRuntimeBootstrap(endpoint: ServerEndpoint, sessionToken: string, resources?: string[]) {
    const response = await ensureSuccess(
      await fetchWithTimeout(
        this.fetchLike,
        new URL("api/system/runtime/bootstrap", endpoint.baseUrl),
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
    );
    return await readJson<TaskAcceptedResponse>(response);
  }
}

import type {
  ErrorEnvelope,
  LauncherReadinessSnapshot,
  LauncherSystemStatusSnapshot,
  ServerEndpoint,
} from "../../shared/launcher-models";

const DEFAULT_REQUEST_TIMEOUT_MS = 5000;

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

  async shutdownFromLauncher(endpoint: ServerEndpoint) {
    await ensureSuccess(
      await fetchWithTimeout(
        this.fetchLike,
        new URL("api/launcher/shutdown", endpoint.baseUrl),
        { method: "POST" },
        this.timeoutMs,
      ),
    );
  }

  async getLauncherStatus(endpoint: ServerEndpoint): Promise<LauncherSystemStatusSnapshot> {
    const response = await ensureSuccess(
      await fetchWithTimeout(
        this.fetchLike,
        new URL("api/launcher/status", endpoint.baseUrl),
        undefined,
        this.timeoutMs,
      ),
    );
    return await readJson<LauncherSystemStatusSnapshot>(response);
  }
}

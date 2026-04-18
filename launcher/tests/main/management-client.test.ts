import { describe, expect, test, vi, afterEach } from "vitest";
import { FetchLauncherManagementClient } from "@main/services/management-client";

describe("FetchLauncherManagementClient", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  test("treats network failures during health probes as unhealthy", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("fetch failed");
      }),
    );

    const client = new FetchLauncherManagementClient();
    const healthy = await client.isHealthy({
      host: "127.0.0.1",
      port: 8080,
      baseUrl: "http://127.0.0.1:8080/",
    });

    expect(healthy).toBe(false);
  });

  test("sends a timeout signal with management fetch requests", async () => {
    let receivedSignal: AbortSignal | undefined;

    vi.stubGlobal(
      "fetch",
      vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
        receivedSignal = init?.signal as AbortSignal | undefined;
        return {
          ok: true,
          json: async () => ({ initialized: true }),
          text: async () => "",
        } satisfies Partial<Response> as Response;
      }),
    );

    const client = new FetchLauncherManagementClient();
    const initialized = await client.getSetupInitialized({
      host: "127.0.0.1",
      port: 8080,
      baseUrl: "http://127.0.0.1:8080/",
    });

    expect(initialized).toBe(true);
    expect(receivedSignal).toBeDefined();
    expect(receivedSignal?.aborted).toBe(false);
  });

  test("reads /readyz payloads even when the server reports 503", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        return {
          ok: false,
          status: 503,
          statusText: "Service Unavailable",
          json: async () => ({ status: "setup_required", reason: "管理员尚未初始化。" }),
          text: async () => "",
        } satisfies Partial<Response> as Response;
      }),
    );

    const client = new FetchLauncherManagementClient();
    const readiness = await client.getReadiness({
      host: "127.0.0.1",
      port: 8080,
      baseUrl: "http://127.0.0.1:8080/",
    });

    expect(readiness.status).toBe("setup_required");
    expect(readiness.reason).toContain("管理员尚未初始化");
  });

  test("creates recovery recheck and runtime bootstrap tasks with auth headers", async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = [];

    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        requests.push({ url: String(input), init });
        return {
          ok: true,
          json: async () => ({ task_id: "task_fixture_0001" }),
          text: async () => "",
        } satisfies Partial<Response> as Response;
      }),
    );

    const client = new FetchLauncherManagementClient();
    const endpoint = {
      host: "127.0.0.1",
      port: 8080,
      baseUrl: "http://127.0.0.1:8080/",
    };

    await client.createRecoveryRecheck(endpoint, "session_fixture_token");
    await client.createRuntimeBootstrap(endpoint, "session_fixture_token", ["chromium", "python-runtime"]);

    expect(requests[0]?.url).toBe("http://127.0.0.1:8080/api/system/recovery/recheck");
    expect(requests[0]?.init?.method).toBe("POST");
    expect((requests[0]?.init?.headers as Record<string, string>).Authorization).toBe("Bearer session_fixture_token");

    expect(requests[1]?.url).toBe("http://127.0.0.1:8080/api/system/runtime/bootstrap");
    expect(requests[1]?.init?.method).toBe("POST");
    expect((requests[1]?.init?.headers as Record<string, string>).Authorization).toBe("Bearer session_fixture_token");
    expect(requests[1]?.init?.body).toBe(JSON.stringify({ resources: ["chromium", "python-runtime"] }));
  });

  test("uses the formal error envelope message for structured failures", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        return {
          ok: false,
          status: 404,
          statusText: "Not Found",
          headers: {
            get: () => "application/json",
          },
          json: async () => ({
            error: {
              code: "platform.resource_missing",
              message: "缺少必要资源",
              details: {
                resource_type: "recovery_summary",
                path: "C:\\RayleaBot\\logs\\recovery-summary.json",
              },
            },
          }),
          text: async () =>
            JSON.stringify({
              error: {
                code: "platform.resource_missing",
                message: "缺少必要资源",
                details: {
                  resource_type: "recovery_summary",
                  path: "C:\\RayleaBot\\logs\\recovery-summary.json",
                },
              },
            }),
        } satisfies Partial<Response> as Response;
      }),
    );

    const client = new FetchLauncherManagementClient();
    const endpoint = {
      host: "127.0.0.1",
      port: 8080,
      baseUrl: "http://127.0.0.1:8080/",
    };

    await expect(client.createRecoveryRecheck(endpoint, "session_fixture_token")).rejects.toThrow("缺少必要资源");
  });
});

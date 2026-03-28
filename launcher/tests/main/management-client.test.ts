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
});

import { describe, expect, test } from "vitest";

describe("launcher dev helpers", () => {
  test("waits for an HTTP response from the Vite renderer server", async () => {
    const { createDevWaitOnOptions } = await import("../../scripts/dev-support.mjs");

    const options = createDevWaitOnOptions("C:\\launcher");

    expect(options.resources).toContain("http-get://127.0.0.1:5174/");
    expect(options.resources).not.toContain("tcp:127.0.0.1:5174");
  });

  test("maps signal-based child exits to a failure status", async () => {
    const { normalizeChildExitCode } = await import("../../scripts/dev-support.mjs");

    expect(normalizeChildExitCode(2, null)).toBe(2);
    expect(normalizeChildExitCode(null, "SIGTERM")).toBe(1);
    expect(normalizeChildExitCode(null, null)).toBe(0);
  });
});

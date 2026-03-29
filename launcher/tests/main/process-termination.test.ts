import { describe, expect, test, vi } from "vitest";
import { terminateProcessId } from "@main/services/process-termination";

describe("terminateProcessId", () => {
  test("treats an already-missing Windows process as a completed termination", async () => {
    const terminated = await terminateProcessId(4242, {
      platform: "win32",
      execFileAsync: vi.fn(async () => {
        throw new Error("ERROR: The process \"4242\" not found.");
      }),
    });

    expect(terminated).toBe(true);
  });

  test("returns false when Windows termination fails for another reason", async () => {
    const terminated = await terminateProcessId(4242, {
      platform: "win32",
      execFileAsync: vi.fn(async () => {
        throw new Error("ERROR: Access is denied.");
      }),
    });

    expect(terminated).toBe(false);
  });
});

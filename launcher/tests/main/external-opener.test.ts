import { afterEach, describe, expect, test, vi } from "vitest";

const shellMocks = vi.hoisted(() => ({
  openExternal: vi.fn(async () => undefined),
  openPath: vi.fn(async () => ""),
}));

vi.mock("electron", () => ({
  shell: shellMocks,
}));

import { externalOpener } from "@main/services/external-opener";

describe("externalOpener", () => {
  afterEach(() => {
    shellMocks.openExternal.mockClear();
    shellMocks.openPath.mockClear();
    shellMocks.openPath.mockResolvedValue("");
  });

  test("throws when Electron reports a directory open failure", async () => {
    shellMocks.openPath.mockResolvedValue("Access is denied.");

    await expect(externalOpener.openDirectory("C:\\RayleaBot\\logs")).rejects.toThrow("Access is denied.");
  });
});

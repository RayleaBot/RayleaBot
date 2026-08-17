import { EventEmitter } from "node:events";
import { describe, expect, test, vi } from "vitest";

import { normalizeChildExitCode, terminateDevProcessTree } from "../../scripts/dev-support.mjs";

describe("launcher development process cleanup", () => {
  test("maps signal-based child exits to a failure status", () => {
    expect(normalizeChildExitCode(2, null)).toBe(2);
    expect(normalizeChildExitCode(null, "SIGTERM")).toBe(1);
    expect(normalizeChildExitCode(null, null)).toBe(0);
  });

  test("uses taskkill tree mode and waits for the Windows child to exit", async () => {
    const child = { pid: 4242, exitCode: null, signalCode: null };
    const killer = new EventEmitter();
    const spawnProcess = vi.fn(() => killer);
    const waitForExit = vi.fn(async () => undefined);

    const termination = terminateDevProcessTree(child, {
      platform: "win32",
      spawnProcess,
      waitForExit,
    });
    killer.emit("exit", 0);
    await termination;

    expect(spawnProcess).toHaveBeenCalledWith(
      "taskkill",
      ["/PID", "4242", "/T", "/F"],
      { stdio: "ignore", windowsHide: true },
    );
    expect(waitForExit).toHaveBeenCalledWith(child);
  });

  test("signals the detached POSIX process group", async () => {
    const child = { pid: 4242, exitCode: null, signalCode: null };
    const killProcess = vi.fn();
    const waitForExit = vi.fn(async () => undefined);

    await terminateDevProcessTree(child, {
      platform: "linux",
      killProcess,
      waitForExit,
    });

    expect(killProcess).toHaveBeenCalledWith(-4242, "SIGTERM");
    expect(waitForExit).toHaveBeenCalledWith(child);
  });

  test("escalates a POSIX process group to SIGKILL after graceful shutdown times out", async () => {
    const child = { pid: 4242, exitCode: null, signalCode: null as string | null };
    const killProcess = vi.fn();
    const waitForExit = vi.fn()
      .mockRejectedValueOnce(new Error("shutdown timeout"))
      .mockImplementationOnce(async () => {
        child.signalCode = "SIGKILL";
      });

    await terminateDevProcessTree(child, {
      platform: "linux",
      killProcess,
      waitForExit,
    });

    expect(killProcess).toHaveBeenNthCalledWith(1, -4242, "SIGTERM");
    expect(killProcess).toHaveBeenNthCalledWith(2, -4242, "SIGKILL");
    expect(waitForExit).toHaveBeenCalledTimes(2);
  });
});

import { describe, expect, test, vi } from "vitest";
import { createApplicationExitManager } from "@main/services/app-exit";

describe("application exit manager", () => {
  test("stops the managed process before quitting", async () => {
    let running = true;
    const stopManagedProcess = vi.fn(async () => {
      running = false;
    });
    const forceKillManagedProcess = vi.fn(async () => {});
    const quitApplication = vi.fn();

    const manager = createApplicationExitManager({
      isManagedProcessRunning: () => running,
      stopManagedProcess,
      forceKillManagedProcess,
      quitApplication,
    });

    await manager.requestExit();

    expect(stopManagedProcess).toHaveBeenCalledTimes(1);
    expect(forceKillManagedProcess).not.toHaveBeenCalled();
    expect(quitApplication).toHaveBeenCalledTimes(1);
    expect(manager.shouldAllowQuit()).toBe(true);
  });

  test("falls back to force kill when coordinated stop fails", async () => {
    let running = true;
    const stopManagedProcess = vi.fn(async () => {
      throw new Error("shutdown failed");
    });
    const forceKillManagedProcess = vi.fn(async () => {
      running = false;
    });
    const quitApplication = vi.fn();

    const manager = createApplicationExitManager({
      isManagedProcessRunning: () => running,
      stopManagedProcess,
      forceKillManagedProcess,
      quitApplication,
    });

    await manager.requestExit();

    expect(stopManagedProcess).toHaveBeenCalledTimes(1);
    expect(forceKillManagedProcess).toHaveBeenCalledTimes(1);
    expect(quitApplication).toHaveBeenCalledTimes(1);
    expect(manager.shouldAllowQuit()).toBe(true);
  });
});

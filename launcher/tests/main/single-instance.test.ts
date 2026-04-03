import { EventEmitter } from "node:events";
import { describe, expect, test, vi } from "vitest";
import { restoreSingleInstanceWindow, wireSingleInstanceLifecycle } from "@main/services/single-instance";

class FakeApp extends EventEmitter {
  constructor(private readonly hasLock: boolean) {
    super();
  }

  quit = vi.fn(() => undefined);
  requestSingleInstanceLock = vi.fn(() => this.hasLock);
}

describe("single-instance lifecycle", () => {
  test("quits immediately when the launcher cannot acquire the single-instance lock", () => {
    const app = new FakeApp(false);

    const acquired = wireSingleInstanceLifecycle(app as never, () => null);

    expect(acquired).toBe(false);
    expect(app.quit).toHaveBeenCalledTimes(1);
  });

  test("restores and focuses the existing window when a second instance is launched", () => {
    const app = new FakeApp(true);
    const window = {
      isMinimized: vi.fn(() => true),
      restore: vi.fn(() => undefined),
      show: vi.fn(() => undefined),
      focus: vi.fn(() => undefined),
    };

    const acquired = wireSingleInstanceLifecycle(app as never, () => window as never);
    app.emit("second-instance");

    expect(acquired).toBe(true);
    expect(window.restore).toHaveBeenCalledTimes(1);
    expect(window.show).toHaveBeenCalledTimes(1);
    expect(window.focus).toHaveBeenCalledTimes(1);
  });

  test("restores a minimized window before showing it", () => {
    const window = {
      isMinimized: vi.fn(() => true),
      restore: vi.fn(() => undefined),
      show: vi.fn(() => undefined),
      focus: vi.fn(() => undefined),
    };

    restoreSingleInstanceWindow(window as never);

    expect(window.restore).toHaveBeenCalledTimes(1);
    expect(window.show).toHaveBeenCalledTimes(1);
    expect(window.focus).toHaveBeenCalledTimes(1);
  });
});

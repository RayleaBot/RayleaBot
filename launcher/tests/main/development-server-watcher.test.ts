import { describe, expect, test } from "vitest";
import {
  NodeDevelopmentServerWatcher,
  consumeDevelopmentServerWatcherProcessId,
} from "@main/services/development-server-watcher";

describe("development server watcher", () => {
  test("consumes a valid watcher PID without retaining it in the environment", () => {
    const environment = {
      RAYLEA_DEV_SERVER_WATCHER_PID: " 12345 ",
    };

    expect(consumeDevelopmentServerWatcherProcessId(environment)).toBe(12345);
    expect(environment).not.toHaveProperty("RAYLEA_DEV_SERVER_WATCHER_PID");
  });

  test("ignores invalid watcher process identifiers", () => {
    const environment = {
      RAYLEA_DEV_SERVER_WATCHER_PID: "not-a-process",
    };

    expect(consumeDevelopmentServerWatcherProcessId(environment)).toBeNull();
    expect(environment).not.toHaveProperty("RAYLEA_DEV_SERVER_WATCHER_PID");
  });

  test("distinguishes a live watcher from an exited process", () => {
    const liveWatcher = new NodeDevelopmentServerWatcher(12345, {
      kill: () => true,
    });
    const exitedWatcher = new NodeDevelopmentServerWatcher(12345, {
      kill: () => {
        throw Object.assign(new Error("missing"), { code: "ESRCH" });
      },
    });

    expect(liveWatcher.isActive()).toBe(true);
    expect(exitedWatcher.isActive()).toBe(false);
  });
});

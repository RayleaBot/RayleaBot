import { afterEach, expect, test, vi } from "vitest";

import { GlassFilterRegistry } from "@renderer/glassSurfaces";

afterEach(() => {
  vi.useRealTimers();
});

test("glass filters for sizes no longer on screen are removed after the release delay", () => {
  vi.useFakeTimers();
  const destroyed: string[] = [];
  const registry = new GlassFilterRegistry<string>(
    (id) => ({ resource: id, ready: Promise.resolve(true) }),
    (resource) => destroyed.push(resource),
    1000,
  );

  const sidebar = registry.acquire("sidebar-720");
  registry.acquire("sidebar-720");
  registry.release("sidebar-720");
  vi.advanceTimersByTime(1000);
  expect(destroyed).toEqual([]);

  registry.release("sidebar-720");
  const resized = registry.acquire("sidebar-640");
  registry.release("sidebar-640");
  vi.advanceTimersByTime(999);
  expect(registry.size).toBe(2);

  // A surface that returns to a size before the delay reuses the same filter.
  expect(registry.acquire("sidebar-720").id).toBe(sidebar.id);
  registry.release("sidebar-720");
  vi.advanceTimersByTime(1000);

  expect(destroyed).toEqual([resized.id, sidebar.id]);
  expect(registry.size).toBe(0);
});

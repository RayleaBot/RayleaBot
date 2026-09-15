// @vitest-environment jsdom
import { afterEach, expect, test, vi } from "vitest";

const workspaceAnimations = vi.hoisted(() => [] as Array<{ stop: ReturnType<typeof vi.fn>; finish: () => void }>);

vi.mock("motion/react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("motion/react")>();
  return {
    ...actual,
    animate: vi.fn(() => {
      let finish!: () => void;
      const finished = new Promise<void>((resolve) => { finish = resolve; });
      const controls = { stop: vi.fn(), then: (resolve: () => void) => finished.then(resolve) };
      workspaceAnimations.push({ stop: controls.stop, finish });
      return controls;
    }),
  };
});

import { animate } from "motion/react";
import { runLauncherWorkspaceTransition, workspaceOffset } from "@renderer/launcherMotion";

afterEach(() => {
  vi.stubGlobal("matchMedia", () => ({ matches: true }));
  runLauncherWorkspaceTransition(() => undefined);
  vi.unstubAllGlobals();
  vi.mocked(animate).mockClear();
  workspaceAnimations.length = 0;
});

test("interrupted navigation continues from the current offset and keeps its replacement active", async () => {
  vi.stubGlobal("matchMedia", () => ({ matches: false }));
  let content = "";

  runLauncherWorkspaceTransition(() => { content = "environment"; });
  expect(workspaceOffset.get()).toBe(8);
  workspaceOffset.set(4);

  runLauncherWorkspaceTransition(() => { content = "diagnostics"; });
  expect(content).toBe("diagnostics");
  expect(workspaceAnimations[0]!.stop).toHaveBeenCalledOnce();
  expect(workspaceOffset.get()).toBe(4);
  expect(animate).toHaveBeenLastCalledWith(workspaceOffset, 0, expect.objectContaining({ duration: 0.22 }));

  workspaceAnimations[0]!.finish();
  await Promise.resolve();
  await Promise.resolve();
  workspaceOffset.set(2);

  runLauncherWorkspaceTransition(() => { content = "settings"; });
  expect(workspaceAnimations[1]!.stop).toHaveBeenCalledOnce();
  expect(workspaceOffset.get()).toBe(2);
});

test("reduced motion applies the latest workspace immediately", () => {
  vi.stubGlobal("matchMedia", () => ({ matches: true }));
  let content = "";

  runLauncherWorkspaceTransition(() => { content = "settings"; });

  expect(content).toBe("settings");
  expect(workspaceOffset.get()).toBe(0);
  expect(animate).not.toHaveBeenCalled();
});

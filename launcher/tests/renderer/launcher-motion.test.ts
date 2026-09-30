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
import { runLauncherWorkspaceTransition, workspaceOpacity } from "@renderer/launcherMotion";

afterEach(() => {
  vi.stubGlobal("matchMedia", () => ({ matches: true }));
  runLauncherWorkspaceTransition(() => undefined);
  vi.unstubAllGlobals();
  vi.mocked(animate).mockClear();
  workspaceAnimations.length = 0;
});

test("interrupted navigation continues from the current opacity and keeps its replacement active", async () => {
  vi.stubGlobal("matchMedia", () => ({ matches: false }));
  let content = "";

  runLauncherWorkspaceTransition(() => { content = "environment"; });
  expect(workspaceOpacity.get()).toBe(0.88);
  workspaceOpacity.set(0.94);

  runLauncherWorkspaceTransition(() => { content = "diagnostics"; });
  expect(content).toBe("diagnostics");
  expect(workspaceAnimations[0]!.stop).toHaveBeenCalledOnce();
  expect(workspaceOpacity.get()).toBe(0.94);
  expect(animate).toHaveBeenLastCalledWith(workspaceOpacity, 1, expect.objectContaining({ duration: 0.22 }));

  workspaceAnimations[0]!.finish();
  await Promise.resolve();
  await Promise.resolve();
  workspaceOpacity.set(0.97);

  runLauncherWorkspaceTransition(() => { content = "settings"; });
  expect(workspaceAnimations[1]!.stop).toHaveBeenCalledOnce();
  expect(workspaceOpacity.get()).toBe(0.97);
});

test("reduced motion applies the latest workspace immediately", () => {
  vi.stubGlobal("matchMedia", () => ({ matches: true }));
  let content = "";

  runLauncherWorkspaceTransition(() => { content = "settings"; });

  expect(content).toBe("settings");
  expect(workspaceOpacity.get()).toBe(1);
  expect(animate).not.toHaveBeenCalled();
});

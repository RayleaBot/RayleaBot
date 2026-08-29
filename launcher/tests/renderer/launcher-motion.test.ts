// @vitest-environment jsdom
import { afterEach, expect, test, vi } from "vitest";
import { runLauncherWorkspaceTransition } from "@renderer/launcherMotion";

afterEach(() => {
  vi.stubGlobal("matchMedia", () => ({ matches: true }));
  runLauncherWorkspaceTransition(() => undefined);
  vi.unstubAllGlobals();
  document.body.innerHTML = "";
});

test("interrupted navigation preserves visible content and cannot cancel its replacement", async () => {
  vi.stubGlobal("matchMedia", () => ({ matches: false }));
  const workspace = document.createElement("main");
  workspace.className = "shell-main";
  document.body.append(workspace);
  const animations: Array<{ cancel: ReturnType<typeof vi.fn>; finish: () => void }> = [];
  const animate = vi.fn(() => {
    let finish!: () => void;
    const finished = new Promise<void>((resolve) => { finish = resolve; });
    const animation = { finished, cancel: vi.fn(), effect: { getComputedTiming: () => ({ progress: 0.5 }) } };
    animations.push({ cancel: animation.cancel, finish });
    return animation;
  });
  Object.defineProperty(workspace, "animate", { value: animate });

  runLauncherWorkspaceTransition(() => { workspace.textContent = "environment"; });
  runLauncherWorkspaceTransition(() => { workspace.textContent = "diagnostics"; });
  expect(workspace.textContent).toBe("diagnostics");
  expect(animations[0]!.cancel).toHaveBeenCalledOnce();
  const replacementFrames = (animate.mock.calls[1] as unknown as [Keyframe[]])[0];
  expect(Number(replacementFrames[0]!.opacity)).toBeGreaterThan(0.85);

  animations[0]!.finish();
  await Promise.resolve();
  await Promise.resolve();
  expect(animations[1]!.cancel).not.toHaveBeenCalled();
  animations[1]!.finish();
});

test("reduced motion applies the latest workspace immediately", () => {
  vi.stubGlobal("matchMedia", () => ({ matches: true }));
  const workspace = document.createElement("main");
  workspace.className = "shell-main";
  document.body.append(workspace);
  const animate = vi.fn();
  Object.defineProperty(workspace, "animate", { value: animate });
  runLauncherWorkspaceTransition(() => { workspace.textContent = "settings"; });
  expect(workspace.textContent).toBe("settings");
  expect(animate).not.toHaveBeenCalled();
});

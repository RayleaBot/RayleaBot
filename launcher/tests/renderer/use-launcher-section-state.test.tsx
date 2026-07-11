// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, test, vi } from "vitest";

import { useLauncherSectionState } from "@renderer/useLauncherSectionState";

function installWorkspaceAnimationMock() {
  const workspace = document.createElement("main");
  workspace.className = "shell-main";
  document.body.append(workspace);
  const animations: Array<{
    cancel: ReturnType<typeof vi.fn>;
    resolve: () => void;
    setProgress: (progress: number | null) => void;
  }> = [];
  const animate = vi.fn((_keyframes: Keyframe[], _options: KeyframeAnimationOptions) => {
    let resolve = () => {};
    const finished = new Promise<void>((next) => { resolve = next; });
    let progress: number | null = 0;
    const cancel = vi.fn();
    const animation = {
      cancel,
      effect: {
        getComputedTiming: () => ({ progress }),
      },
      finished,
    } as unknown as Animation;
    animations.push({
      cancel,
      resolve,
      setProgress: (nextProgress) => { progress = nextProgress; },
    });
    return animation;
  });
  Object.defineProperty(HTMLElement.prototype, "animate", {
    configurable: true,
    value: animate,
  });
  return { animate, animations };
}

describe("useLauncherSectionState", () => {
  afterEach(() => {
    Reflect.deleteProperty(HTMLElement.prototype, "animate");
    document.querySelector(".shell-main")?.remove();
    delete document.documentElement.dataset.launcherViewTransitionKind;
    vi.unstubAllGlobals();
  });

  test("switches the workspace immediately and starts a non-blocking animation", async () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: false })));
    const { animate, animations } = installWorkspaceAnimationMock();
    const { result } = renderHook(() => useLauncherSectionState());

    act(() => result.current.setActiveSection("environment"));

    expect(result.current.activeSection).toBe("environment");
    expect(animate).toHaveBeenCalledWith(
      [{ opacity: 0 }, { opacity: 1 }],
      expect.objectContaining({
        duration: 300,
        easing: "cubic-bezier(0.25, 0.1, 0.25, 1)",
      }),
    );
    await act(async () => {
      animations[0]?.resolve();
      await Promise.resolve();
    });
  });

  test("switches immediately when reduced motion is requested", () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true })));
    const { animate } = installWorkspaceAnimationMock();
    const { result } = renderHook(() => useLauncherSectionState());

    act(() => result.current.setActiveSection("diagnostics"));

    expect(result.current.activeSection).toBe("diagnostics");
    expect(animate).not.toHaveBeenCalled();
  });

  test("cancels the running animation while applying every navigation immediately", async () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: false })));
    const { animate, animations } = installWorkspaceAnimationMock();
    const { result } = renderHook(() => useLauncherSectionState());

    act(() => result.current.setActiveSection("diagnostics"));
    animations[0]?.setProgress(0.5);
    act(() => result.current.setActiveSection("settings"));
    act(() => result.current.setActiveSection("about"));

    expect(result.current.activeSection).toBe("about");
    expect(animate).toHaveBeenCalledTimes(3);
    expect(animations[0]?.cancel).toHaveBeenCalledOnce();
    expect(animations[1]?.cancel).toHaveBeenCalledOnce();
    expect(animate.mock.calls[1]?.[0]).toEqual([{ opacity: 0.5 }, { opacity: 1 }]);
    await act(async () => {
      animations[2]?.resolve();
      await Promise.resolve();
    });
  });
});

// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, test, vi } from "vitest";

import { useLauncherSectionState } from "@renderer/useLauncherSectionState";

describe("useLauncherSectionState", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  test("switches rendered section with one 200ms content transition", async () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useLauncherSectionState());

    act(() => {
      result.current.setActiveSection("environment");
    });

    expect(result.current.activeSection).toBe("environment");
    expect(result.current.renderedSection).toBe("environment");
    expect(result.current.sectionTransitionState).toBe("entering");

    await act(async () => {
      vi.advanceTimersByTime(200);
    });

    expect(result.current.sectionTransitionState).toBe("idle");
  });

  test("switches immediately when reduced motion is requested", () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true })));
    const { result } = renderHook(() => useLauncherSectionState());

    act(() => {
      result.current.setActiveSection("diagnostics");
    });

    expect(result.current.renderedSection).toBe("diagnostics");
    expect(result.current.sectionTransitionState).toBe("idle");
  });

  test("restarts the transition window when navigation changes rapidly", async () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useLauncherSectionState());

    act(() => {
      result.current.setActiveSection("diagnostics");
      vi.advanceTimersByTime(120);
      result.current.setActiveSection("about");
      vi.advanceTimersByTime(80);
    });

    expect(result.current.renderedSection).toBe("about");
    expect(result.current.sectionTransitionState).toBe("entering");

    await act(async () => {
      vi.advanceTimersByTime(120);
    });

    expect(result.current.sectionTransitionState).toBe("idle");
  });
});

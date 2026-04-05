// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";

import { useLauncherSectionState } from "@renderer/useLauncherSectionState";

describe("useLauncherSectionState", () => {
  test("switches rendered section through exiting and entering states", async () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useLauncherSectionState());

    act(() => {
      result.current.setActiveSection("environment");
    });

    expect(result.current.activeSection).toBe("environment");
    expect(result.current.renderedSection).toBe("status");
    expect(result.current.sectionTransitionState).toBe("exiting");

    await act(async () => {
      vi.advanceTimersByTime(90);
    });

    expect(result.current.renderedSection).toBe("environment");
    expect(result.current.sectionTransitionState).toBe("entering");

    await act(async () => {
      vi.advanceTimersByTime(180);
    });

    expect(result.current.sectionTransitionState).toBe("idle");
    vi.useRealTimers();
  });
});

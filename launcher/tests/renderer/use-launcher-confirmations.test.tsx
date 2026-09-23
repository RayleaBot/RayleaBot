// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, test, vi } from "vitest";
import { useLauncherConfirmations } from "@renderer/useLauncherConfirmations";
import type { LauncherDesktopApi } from "@shared/desktop-api";

afterEach(() => { Reflect.deleteProperty(window, "rayleaLauncher"); });

describe("useLauncherConfirmations", () => {
  test.each(["close", "external-stop"] as const)("does not reopen a resolved %s prompt when the pending lookup arrives late", async (kind) => {
    let showClose!: () => void;
    let showExternal!: () => void;
    let resolvePending!: (pending: boolean) => void;
    const pending = new Promise<boolean>((resolve) => { resolvePending = resolve; });
    const unsubscribeClose = vi.fn();
    const unsubscribeExternal = vi.fn();
    const closeConfirmResponse = vi.fn(async () => undefined);
    const externalStopConfirmResponse = vi.fn(async () => undefined);
    Object.defineProperty(window, "rayleaLauncher", {
      configurable: true,
      value: {
        onShowExitConfirm: (listener: () => void) => { showClose = listener; return unsubscribeClose; },
        onShowExternalStopConfirm: (listener: () => void) => { showExternal = listener; return unsubscribeExternal; },
        hasPendingCloseConfirm: () => kind === "close" ? pending : Promise.resolve(false),
        hasPendingExternalStopConfirm: () => kind === "external-stop" ? pending : Promise.resolve(false),
        closeConfirmResponse,
        externalStopConfirmResponse,
      } as unknown as LauncherDesktopApi,
    });
    const setSnapshot = vi.fn();
    const runAction = vi.fn();
    const { result, unmount } = renderHook(() => useLauncherConfirmations(setSnapshot, runAction));
    await act(async () => {
      if (kind === "close") showClose();
      else showExternal();
    });
    expect(kind === "close" ? result.current.exitConfirmOpen : result.current.confirmedAction === "stop-external").toBe(true);
    await act(async () => {
      if (kind === "close") result.current.handleExitConfirmClose();
      else result.current.handleConfirmedActionCancel();
    });
    await act(async () => { resolvePending(true); });
    expect(result.current.exitConfirmOpen).toBe(false);
    expect(result.current.confirmedAction).toBeNull();
    if (kind === "close") expect(closeConfirmResponse).toHaveBeenCalledWith({ action: "cancel", setAsDefault: false });
    else expect(externalStopConfirmResponse).toHaveBeenCalledWith(false);
    unmount();
    expect(unsubscribeClose).toHaveBeenCalledOnce();
    expect(unsubscribeExternal).toHaveBeenCalledOnce();
  });
});

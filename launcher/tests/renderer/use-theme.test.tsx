// @vitest-environment jsdom
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import type { LauncherDesktopApi } from "@shared/desktop-api";
import { ThemeProvider, useTheme } from "@renderer/useTheme";

function installDesktopApi(setThemeMode = vi.fn(async () => {})) {
  Object.defineProperty(window, "rayleaLauncher", {
    configurable: true,
    value: { setThemeMode } as LauncherDesktopApi,
  });
  return setThemeMode;
}

function ThemeProbe() {
  const { mode, effectiveTheme, setMode, syncError } = useTheme();
  return (
    <div>
      <span>{mode}:{effectiveTheme}</span>
      <button onClick={() => setMode("dark")}>深色</button>
      {syncError ? <span>{syncError}</span> : null}
    </div>
  );
}

describe("ThemeProvider", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    Reflect.deleteProperty(document, "startViewTransition");
    delete document.documentElement.dataset.launcherViewTransitionKind;
  });

  test("defaults to the system theme and follows system changes", async () => {
    let listener: (() => void) | undefined;
    const media = {
      matches: false,
      addEventListener: vi.fn((_name: string, next: () => void) => { listener = next; }),
      removeEventListener: vi.fn(),
    };
    vi.stubGlobal("matchMedia", vi.fn(() => media));
    const setThemeMode = installDesktopApi();

    render(<ThemeProvider><ThemeProbe /></ThemeProvider>);

    expect(screen.getByText("system:light")).toBeInTheDocument();
    await waitFor(() => expect(setThemeMode).toHaveBeenCalledWith("system"));
    expect(document.documentElement.dataset.theme).toBe("light");

    media.matches = true;
    act(() => listener?.());
    expect(screen.getByText("system:dark")).toBeInTheDocument();
    expect(document.documentElement.style.colorScheme).toBe("dark");
  });

  test("persists explicit choices and reports native synchronization failures", async () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })));
    window.localStorage.setItem("raylea-theme-mode", "light");
    const setThemeMode = installDesktopApi(vi.fn(async () => {
      throw new Error("IPC unavailable");
    }));

    render(<ThemeProvider><ThemeProbe /></ThemeProvider>);
    expect(screen.getByText("light:light")).toBeInTheDocument();

    screen.getByRole("button", { name: "深色" }).click();

    expect(window.localStorage.getItem("raylea-theme-mode")).toBe("dark");
    await waitFor(() => expect(setThemeMode).toHaveBeenLastCalledWith("dark"));
    expect(await screen.findByText("窗口主题同步失败，界面主题仍已保留。")).toBeInTheDocument();
  });

  test("applies theme changes through a managed view transition", () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })));
    installDesktopApi();
    let resolveFinished = () => {};
    const finished = new Promise<void>((resolve) => { resolveFinished = resolve; });
    const startViewTransition = vi.fn((update: () => void) => {
      update();
      return {
        finished,
        ready: Promise.resolve(),
        updateCallbackDone: Promise.resolve(),
        skipTransition: vi.fn(),
      } as ViewTransition;
    });
    Object.defineProperty(document, "startViewTransition", {
      configurable: true,
      value: startViewTransition,
    });

    render(<ThemeProvider><ThemeProbe /></ThemeProvider>);
    fireEvent.click(screen.getByRole("button", { name: "深色" }));

    expect(screen.getByText("dark:dark")).toBeInTheDocument();
    expect(startViewTransition).toHaveBeenCalledOnce();
    expect(document.documentElement).toHaveAttribute(
      "data-launcher-view-transition-kind",
      "theme",
    );
    act(() => resolveFinished());
  });

  test("applies theme changes immediately when reduced motion is requested", () => {
    vi.stubGlobal("matchMedia", vi.fn((query: string) => ({
      matches: query.includes("prefers-reduced-motion"),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })));
    installDesktopApi();
    const startViewTransition = vi.fn();
    Object.defineProperty(document, "startViewTransition", {
      configurable: true,
      value: startViewTransition,
    });

    render(<ThemeProvider><ThemeProbe /></ThemeProvider>);
    fireEvent.click(screen.getByRole("button", { name: "深色" }));

    expect(screen.getByText("dark:dark")).toBeInTheDocument();
    expect(startViewTransition).not.toHaveBeenCalled();
  });
});

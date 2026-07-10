// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import type { LauncherDesktopApi } from "@shared/desktop-api";
import { ThemeModeMenu } from "@renderer/ThemeModeMenu";
import { ThemeProvider } from "@renderer/useTheme";

function setupMatchMedia(reducedMotion: boolean) {
  vi.stubGlobal("matchMedia", vi.fn((query: string) => ({
    matches: query.includes("reduced-motion") ? reducedMotion : false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })));
}

function renderMenu() {
  Object.defineProperty(window, "rayleaLauncher", {
    configurable: true,
    value: { setThemeMode: vi.fn(async () => {}) } as LauncherDesktopApi,
  });
  return render(<ThemeProvider><ThemeModeMenu /></ThemeProvider>);
}

describe("ThemeModeMenu", () => {
  beforeEach(() => {
    window.localStorage.clear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  test("acknowledges a selection before closing and commits the theme after exit", () => {
    setupMatchMedia(false);
    renderMenu();

    fireEvent.click(screen.getByRole("button", { name: "主题：跟随系统" }));
    expect(screen.getByRole("menuitemradio", { name: "跟随系统" })).toBeInTheDocument();
    expect(screen.getByRole("menuitemradio", { name: "浅色" })).toBeInTheDocument();
    expect(screen.getByRole("menuitemradio", { name: "深色" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("menuitemradio", { name: "深色" }));
    expect(screen.getByRole("menuitemradio", { name: "深色" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(screen.getByRole("button", { name: "主题：跟随系统" })).toBeInTheDocument();

    const surface = document.querySelector<HTMLElement>(".theme-menu-surface");
    expect(surface).toHaveAttribute("data-state", "closing");
    expect(surface?.parentElement).toHaveClass("theme-menu-positioner");
    expect(surface?.parentElement).not.toHaveAttribute("data-state");
    act(() => vi.advanceTimersByTime(179));
    expect(screen.getByRole("menuitemradio", { name: "深色" })).toBeInTheDocument();
    act(() => vi.advanceTimersByTime(1));
    expect(screen.queryByRole("menuitemradio", { name: "深色" })).not.toBeInTheDocument();
    act(() => vi.advanceTimersByTime(1));
    expect(screen.getByRole("button", { name: "主题：深色" })).toHaveFocus();
  });

  test("keeps a cancelled menu mounted until its exit animation completes", () => {
    setupMatchMedia(false);
    renderMenu();

    const trigger = screen.getByRole("button", { name: "主题：跟随系统" });
    fireEvent.click(trigger);
    fireEvent.click(trigger);

    expect(screen.getByRole("menuitemradio", { name: "浅色" })).toBeInTheDocument();
    const surface = document.querySelector<HTMLElement>(".theme-menu-surface");
    expect(surface).toHaveAttribute("data-state", "closing");

    act(() => vi.advanceTimersByTime(180));
    expect(screen.queryByRole("menuitemradio", { name: "浅色" })).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  test("allows the first theme option to be selected", () => {
    setupMatchMedia(false);
    window.localStorage.setItem("raylea-theme-mode", "light");
    renderMenu();

    fireEvent.click(screen.getByRole("button", { name: "主题：浅色" }));
    fireEvent.click(screen.getByRole("menuitemradio", { name: "跟随系统" }));
    act(() => vi.advanceTimersByTime(180));
    act(() => vi.advanceTimersByTime(1));

    expect(screen.getByRole("button", { name: "主题：跟随系统" })).toHaveFocus();
  });

  test("closes immediately when reduced motion is requested", () => {
    setupMatchMedia(true);
    renderMenu();

    fireEvent.click(screen.getByRole("button", { name: "主题：跟随系统" }));
    fireEvent.click(screen.getByRole("menuitemradio", { name: "浅色" }));

    expect(screen.queryByRole("menuitemradio", { name: "浅色" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "主题：浅色" })).toHaveFocus();
  });
});

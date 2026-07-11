// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  test("commits a selection after the presence surface closes", async () => {
    setupMatchMedia(false);
    renderMenu();

    fireEvent.click(screen.getByRole("button", { name: "主题：跟随系统" }));
    expect(screen.getByRole("menuitemradio", { name: "跟随系统" })).toBeInTheDocument();
    expect(screen.getByRole("menuitemradio", { name: "浅色" })).toBeInTheDocument();
    expect(screen.getByRole("menuitemradio", { name: "深色" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("menuitemradio", { name: "深色" }));

    await waitFor(() => {
      expect(screen.queryByRole("menuitemradio", { name: "深色" })).not.toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "主题：深色" })).toHaveFocus();
  });

  test("closes a cancelled menu and restores trigger focus", async () => {
    setupMatchMedia(false);
    renderMenu();

    const trigger = screen.getByRole("button", { name: "主题：跟随系统" });
    fireEvent.click(trigger);
    fireEvent.click(trigger);

    await waitFor(() => {
      expect(screen.queryByRole("menuitemradio", { name: "浅色" })).not.toBeInTheDocument();
    });
    expect(trigger).toHaveFocus();
  });

  test("allows the first theme option to be selected", async () => {
    setupMatchMedia(false);
    window.localStorage.setItem("raylea-theme-mode", "light");
    renderMenu();

    fireEvent.click(screen.getByRole("button", { name: "主题：浅色" }));
    fireEvent.click(screen.getByRole("menuitemradio", { name: "跟随系统" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "主题：跟随系统" })).toHaveFocus();
    });
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

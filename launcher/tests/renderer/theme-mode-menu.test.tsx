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

  test("commits a selection and closes the menu", async () => {
    setupMatchMedia(false);
    renderMenu();

    fireEvent.click(screen.getByRole("button", { name: "主题：跟随系统" }));
    fireEvent.click(screen.getByRole("menuitemradio", { name: "深色" }));

    await waitFor(() => {
      expect(screen.queryByRole("menuitemradio", { name: "深色" })).not.toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "主题：深色" })).toBeInTheDocument();
  });
});

// @vitest-environment jsdom
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { defineComponent, h } from "vue";
import { System } from "@wailsio/runtime";
import type { LauncherDesktopApi } from "@shared/desktop-api";
import ThemeModeMenu from "@renderer/ThemeModeMenu.vue";
import { provideTheme, useTheme } from "@renderer/useTheme";

import { findButton, getButton } from "../helpers/dom";

vi.mock("@wailsio/runtime", () => ({
  System: { IsLinux: vi.fn(() => false) },
}));

function installDesktopApi(setThemeMode = vi.fn(async () => {})) {
  Object.defineProperty(window, "rayleaLauncher", {
    configurable: true,
    value: { setThemeMode } as unknown as LauncherDesktopApi,
  });
  return setThemeMode;
}

function installViewTransitions() {
  const start = vi.fn((update: () => void | Promise<void>) => {
    const updated = Promise.resolve().then(update);
    return { ready: updated, finished: updated, updateCallbackDone: updated, skipTransition: vi.fn() };
  });
  Object.defineProperty(document, "startViewTransition", { configurable: true, value: start });
  return start;
}

const ThemeProbe = defineComponent(() => {
  const { mode, effectiveTheme, setMode, syncError } = useTheme();
  return () => h("div", [
    h("span", { "data-probe": "" }, `${mode.value}:${effectiveTheme.value}`),
    h("button", { type: "button", onClick: () => setMode("dark") }, "深色"),
    syncError.value ? h("span", { role: "status" }, syncError.value) : null,
  ]);
});

function themed(component: ReturnType<typeof defineComponent>) {
  return defineComponent(() => {
    provideTheme();
    return () => h(component);
  });
}

let wrapper: VueWrapper | undefined;

beforeEach(() => {
  window.localStorage.clear();
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = "";
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  Reflect.deleteProperty(window, "rayleaLauncher");
  Reflect.deleteProperty(document, "startViewTransition");
});

describe("theme", () => {
  test.each([
    { platform: "Linux", isLinux: true, transitionCount: 0 },
    { platform: "other desktop platforms", isLinux: false, transitionCount: 1 },
  ])("switches and persists themes with platform-compatible transitions on $platform", async ({ isLinux, transitionCount }) => {
    vi.spyOn(System, "IsLinux").mockReturnValue(isLinux);
    vi.stubGlobal("matchMedia", vi.fn(() => ({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })));
    const startViewTransition = installViewTransitions();
    const setThemeMode = installDesktopApi();
    wrapper = mount(themed(ThemeProbe));

    await wrapper.get("button").trigger("click");
    await flushPromises();

    expect(startViewTransition).toHaveBeenCalledTimes(transitionCount);
    expect(wrapper.get("[data-probe]").text()).toBe("dark:dark");
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(window.localStorage.getItem("raylea-theme-mode")).toBe("dark");
    expect(setThemeMode).toHaveBeenLastCalledWith("dark");
  });

  test("defaults to the system theme and follows system changes without Linux view transitions", async () => {
    vi.spyOn(System, "IsLinux").mockReturnValue(true);
    const startViewTransition = installViewTransitions();
    let listener: (() => void) | undefined;
    const media = {
      matches: false,
      addEventListener: vi.fn((_name: string, next: () => void) => { listener = next; }),
      removeEventListener: vi.fn(),
    };
    vi.stubGlobal("matchMedia", vi.fn((query: string) => query === "(prefers-color-scheme: dark)"
      ? media
      : { matches: false }));
    const setThemeMode = installDesktopApi();

    wrapper = mount(themed(ThemeProbe));

    expect(wrapper.get("[data-probe]").text()).toBe("system:light");
    expect(setThemeMode).toHaveBeenCalledWith("system");
    expect(document.documentElement.dataset.theme).toBe("light");

    media.matches = true;
    listener?.();
    await wrapper.vm.$nextTick();
    expect(wrapper.get("[data-probe]").text()).toBe("system:dark");
    expect(document.documentElement.style.colorScheme).toBe("dark");
    expect(setThemeMode).toHaveBeenCalledTimes(2);
    expect(startViewTransition).not.toHaveBeenCalled();
  });

  test("persists explicit choices and reports native synchronization failures", async () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })));
    window.localStorage.setItem("raylea-theme-mode", "light");
    const setThemeMode = installDesktopApi(vi.fn(async () => {
      throw new Error("desktop bridge unavailable");
    }));

    wrapper = mount(themed(ThemeProbe));
    expect(wrapper.get("[data-probe]").text()).toBe("light:light");

    await wrapper.get("button").trigger("click");

    expect(wrapper.get("[data-probe]").text()).toBe("dark:dark");
    expect(window.localStorage.getItem("raylea-theme-mode")).toBe("dark");
    expect(setThemeMode).toHaveBeenLastCalledWith("dark");
    await vi.waitFor(() => expect(wrapper?.get("[role=status]").text()).toBe("窗口主题同步失败，界面主题仍已保留。"));
  });

  test("commits a menu selection and closes the menu", async () => {
    vi.stubGlobal("matchMedia", vi.fn((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })));
    installDesktopApi();
    wrapper = mount(themed(ThemeModeMenu), { attachTo: document.body });

    getButton("主题：跟随系统").click();
    const darkItem = await vi.waitFor(() => {
      const item = [...document.querySelectorAll<HTMLElement>("[role=menuitemradio]")].find((candidate) => candidate.textContent?.trim() === "深色");
      if (!item) throw new Error("menu is not open");
      return item;
    });
    darkItem.click();

    await vi.waitFor(() => expect(document.querySelector("[role=menuitemradio]")).toBeNull());
    await findButton("主题：深色");
  });
});

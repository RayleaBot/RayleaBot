// @vitest-environment jsdom
import { mount, type VueWrapper } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { defineComponent, h } from "vue";
import type { LauncherDesktopApi } from "@shared/desktop-api";
import ThemeModeMenu from "@renderer/ThemeModeMenu.vue";
import { provideTheme, useTheme } from "@renderer/useTheme";

import { findButton, getButton } from "../helpers/dom";

function installDesktopApi(setThemeMode = vi.fn(async () => {})) {
  Object.defineProperty(window, "rayleaLauncher", {
    configurable: true,
    value: { setThemeMode } as unknown as LauncherDesktopApi,
  });
  return setThemeMode;
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
});

describe("theme", () => {
  test("defaults to the system theme and follows system changes", async () => {
    let listener: (() => void) | undefined;
    const media = {
      matches: false,
      addEventListener: vi.fn((_name: string, next: () => void) => { listener = next; }),
      removeEventListener: vi.fn(),
    };
    vi.stubGlobal("matchMedia", vi.fn(() => media));
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

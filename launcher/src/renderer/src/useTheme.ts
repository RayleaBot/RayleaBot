import { inject, onScopeDispose, provide, readonly, ref, watch, type InjectionKey, type Ref } from "vue";
import {
  isLauncherThemeMode,
  resolveLauncherEffectiveTheme,
  type LauncherEffectiveTheme,
  type LauncherThemeMode,
} from "@shared/launcher-theme";
import { applyLauncherDocumentTheme } from "./launcherTheme";
import { runLauncherViewTransition, type ThemeMotionOrigin } from "./launcherMotion";

type ThemeMode = LauncherThemeMode;

const syncFailedText = "窗口主题同步失败，界面主题仍已保留。";

function resolveSystemTheme(): LauncherEffectiveTheme {
  if (typeof window === "undefined" || !window.matchMedia) {
    return "light";
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function readStoredMode(): ThemeMode {
  if (typeof window === "undefined") {
    return "system";
  }
  const stored = window.localStorage.getItem("raylea-theme-mode");
  if (isLauncherThemeMode(stored)) {
    return stored;
  }
  return "system";
}

function writeStoredMode(mode: ThemeMode) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem("raylea-theme-mode", mode);
}

export interface ThemeController {
  mode: Readonly<Ref<ThemeMode>>;
  effectiveTheme: Readonly<Ref<LauncherEffectiveTheme>>;
  /** Changes the theme; with an origin, the new theme grows out of that viewport point. */
  setMode: (mode: ThemeMode, origin?: ThemeMotionOrigin) => void;
  syncError: Readonly<Ref<string | null>>;
}

const themeKey: InjectionKey<ThemeController> = Symbol("launcher-theme");

/** Owns the theme for the component tree below the caller and keeps the native window in step with it. */
export function provideTheme(): ThemeController {
  const mode = ref<ThemeMode>(readStoredMode());
  const effectiveTheme = ref<LauncherEffectiveTheme>(resolveLauncherEffectiveTheme(mode.value, resolveSystemTheme() === "dark"));
  const syncError = ref<string | null>(null);
  let targetTheme = effectiveTheme.value;
  applyLauncherDocumentTheme(effectiveTheme.value);

  const transitionToEffectiveTheme = (nextTheme: LauncherEffectiveTheme, origin?: ThemeMotionOrigin) => {
    if (targetTheme === nextTheme) {
      return;
    }
    targetTheme = nextTheme;
    runLauncherViewTransition("theme", () => {
      effectiveTheme.value = nextTheme;
      applyLauncherDocumentTheme(nextTheme);
    }, origin);
  };

  const setMode = (next: ThemeMode, origin?: ThemeMotionOrigin) => {
    writeStoredMode(next);
    mode.value = next;
    transitionToEffectiveTheme(resolveLauncherEffectiveTheme(next, resolveSystemTheme() === "dark"), origin);
  };

  // A failure is reported only for the latest mode while the tree is mounted.
  let syncRequest = 0;
  watch(mode, (value) => {
    const request = ++syncRequest;
    syncError.value = null;
    void window.rayleaLauncher.setThemeMode(value).catch(() => {
      if (request === syncRequest) {
        syncError.value = syncFailedText;
      }
    });
  }, { immediate: true });

  const systemTheme = typeof window === "undefined" ? undefined : window.matchMedia?.("(prefers-color-scheme: dark)");
  const followSystemTheme = () => {
    if (mode.value !== "system" || !systemTheme) {
      return;
    }
    transitionToEffectiveTheme(resolveLauncherEffectiveTheme("system", systemTheme.matches));
    void window.rayleaLauncher.setThemeMode("system").catch(() => {
      syncError.value = syncFailedText;
    });
  };
  systemTheme?.addEventListener("change", followSystemTheme);
  onScopeDispose(() => {
    syncRequest += 1;
    systemTheme?.removeEventListener("change", followSystemTheme);
  });

  const controller: ThemeController = {
    mode: readonly(mode),
    effectiveTheme: readonly(effectiveTheme),
    setMode,
    syncError: readonly(syncError),
  };
  provide(themeKey, controller);
  return controller;
}

export function useTheme(): ThemeController {
  return inject(themeKey)!;
}

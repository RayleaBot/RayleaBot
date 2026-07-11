import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import {
  isLauncherThemeMode,
  resolveLauncherEffectiveTheme,
  type LauncherEffectiveTheme,
  type LauncherThemeMode,
} from "@shared/launcher-theme";
import { applyLauncherDocumentTheme } from "./launcherTheme";
import { runLauncherViewTransition } from "./launcherMotion";

export type ThemeMode = LauncherThemeMode;

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

export interface ThemeContextValue {
  mode: ThemeMode;
  effectiveTheme: LauncherEffectiveTheme;
  setMode: (mode: ThemeMode) => void;
  syncError: string | null;
}

const ThemeContext = createContext<ThemeContextValue>({
  mode: "system",
  effectiveTheme: "light",
  setMode: () => {},
  syncError: null,
});

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [mode, setModeState] = useState<ThemeMode>(readStoredMode);
  const [effectiveTheme, setEffectiveTheme] = useState<LauncherEffectiveTheme>(() =>
    resolveLauncherEffectiveTheme(readStoredMode(), resolveSystemTheme() === "dark"),
  );
  const [syncError, setSyncError] = useState<string | null>(null);
  const effectiveThemeRef = useRef(effectiveTheme);

  const transitionToEffectiveTheme = useCallback((nextTheme: LauncherEffectiveTheme) => {
    if (effectiveThemeRef.current === nextTheme) {
      return;
    }
    effectiveThemeRef.current = nextTheme;

    if (typeof document === "undefined") {
      setEffectiveTheme(nextTheme);
      return;
    }
    runLauncherViewTransition("theme", () => setEffectiveTheme(nextTheme));
  }, []);

  const setMode = useCallback((next: ThemeMode) => {
    writeStoredMode(next);
    setModeState(next);
    transitionToEffectiveTheme(
      resolveLauncherEffectiveTheme(next, resolveSystemTheme() === "dark"),
    );
  }, [transitionToEffectiveTheme]);

  useLayoutEffect(() => {
    effectiveThemeRef.current = effectiveTheme;
    applyLauncherDocumentTheme(effectiveTheme);
  }, [effectiveTheme]);

  useEffect(() => {
    let active = true;
    setSyncError(null);
    void window.rayleaLauncher.setThemeMode(mode).catch(() => {
      if (active) {
        setSyncError("窗口主题同步失败，界面主题仍已保留。");
      }
    });
    return () => {
      active = false;
    };
  }, [mode]);

  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) {
      return;
    }
    const mql = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => {
      if (mode === "system") {
        transitionToEffectiveTheme(resolveLauncherEffectiveTheme("system", mql.matches));
      }
    };
    mql.addEventListener("change", handler);
    return () => mql.removeEventListener("change", handler);
  }, [mode, transitionToEffectiveTheme]);

  return (
    <ThemeContext.Provider value={{ mode, effectiveTheme, setMode, syncError }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme(): ThemeContextValue {
  return useContext(ThemeContext);
}

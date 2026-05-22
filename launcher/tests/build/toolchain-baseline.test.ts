import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";

type LauncherPackageJson = {
  dependencies?: Record<string, string>;
  devDependencies?: Record<string, string>;
};

const launcherPackage = JSON.parse(
  readFileSync(new URL("../../package.json", import.meta.url), "utf8"),
) as LauncherPackageJson;

const engineeringBaseline = readFileSync(
  new URL("../../../docs/engineering/baseline.md", import.meta.url),
  "utf8",
);

describe("launcher toolchain baseline", () => {
  test("pins the current official Vite React plugin line", () => {
    expect(launcherPackage.devDependencies?.["@vitejs/plugin-react"]).toBe("6.0.1");
  });

  test("pins the launcher Vite toolchain line", () => {
    expect(launcherPackage.devDependencies?.vite).toBe("8.0.10");
    expect(launcherPackage.devDependencies?.vitest).toBe("4.1.5");
    expect(launcherPackage.devDependencies?.["@vitest/coverage-v8"]).toBe("4.1.5");
  });

  test("records the launcher react plugin baseline in engineering docs", () => {
    expect(engineeringBaseline).toContain("Vite `8.0.10`");
    expect(engineeringBaseline).toContain("`@vitejs/plugin-react 6.0.1`");
  });

  test("pins semver as a direct runtime dependency for release metadata comparison", () => {
    expect(launcherPackage.dependencies?.semver).toBe("7.8.1");
  });

  test("does not keep an unused concurrently dependency in the launcher toolchain", () => {
    expect(launcherPackage.devDependencies?.concurrently).toBeUndefined();
  });
});

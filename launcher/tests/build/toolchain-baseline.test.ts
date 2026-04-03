import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";

type LauncherPackageJson = {
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

  test("records the launcher react plugin baseline in engineering docs", () => {
    expect(engineeringBaseline).toContain("`@vitejs/plugin-react 6.0.1`");
  });
});

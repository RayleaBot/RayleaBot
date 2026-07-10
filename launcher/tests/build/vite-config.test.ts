import { describe, expect, test } from "vitest";
import viteConfig from "../../vite.config";
import preloadViteConfig from "../../vite.preload.config";

describe("vite renderer packaging config", () => {
  test("uses root-relative assets for the standard packaged renderer protocol", () => {
    expect(viteConfig.base).toBe("/");
  });

  test("bundles the sandboxed preload into one CommonJS file", () => {
    expect(preloadViteConfig.build?.lib).toMatchObject({
      formats: ["cjs"],
    });
    expect(preloadViteConfig.build?.outDir).toMatch(/dist[\\/]preload$/);
    expect(preloadViteConfig.build?.sourcemap).toBe(false);
  });
});

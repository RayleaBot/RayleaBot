import { defineConfig } from "vitest/config";
import path from "node:path";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@renderer": path.resolve(import.meta.dirname, "src/renderer/src"),
      "@shared": path.resolve(import.meta.dirname, "src/shared"),
    },
  },
  test: {
    globals: true,
    environment: "node",
    include: ["tests/renderer/**/*.test.ts", "tests/shared/**/*.test.ts", "tests/scripts/**/*.test.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text-summary"],
      thresholds: {
        statements: 35,
        lines: 35,
        functions: 35,
        branches: 20,
      },
    },
  },
});

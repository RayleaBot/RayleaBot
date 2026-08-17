import { defineConfig } from "vitest/config";
import path from "node:path";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@renderer": path.resolve(__dirname, "src/renderer/src"),
      "@shared": path.resolve(__dirname, "src/shared"),
    },
  },
  test: {
    globals: true,
    environment: "node",
    setupFiles: ["./tests/setup.ts"],
    include: ["tests/renderer/**/*.test.ts", "tests/renderer/**/*.test.tsx", "tests/shared/**/*.test.ts", "tests/scripts/**/*.test.ts"],
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

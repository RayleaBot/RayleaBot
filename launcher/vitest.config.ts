import { defineConfig } from "vitest/config";
import path from "node:path";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@main": path.resolve(__dirname, "src/main"),
      "@preload": path.resolve(__dirname, "src/preload"),
      "@renderer": path.resolve(__dirname, "src/renderer/src"),
      "@shared": path.resolve(__dirname, "src/shared"),
    },
  },
  test: {
    globals: true,
    environment: "node",
    include: ["tests/**/*.test.ts"],
  },
});

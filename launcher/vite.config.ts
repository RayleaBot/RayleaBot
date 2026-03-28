import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import path from "node:path";

export default defineConfig({
  root: "src/renderer",
  base: "./",
  plugins: [vue()],
  resolve: {
    alias: {
      "@renderer": path.resolve(__dirname, "src/renderer/src"),
      "@shared": path.resolve(__dirname, "src/shared"),
    },
  },
  build: {
    outDir: path.resolve(__dirname, "dist/renderer"),
    emptyOutDir: true,
  },
  server: {
    host: "127.0.0.1",
    port: 5174,
    strictPort: true,
  },
});

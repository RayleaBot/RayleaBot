import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import path from "node:path";

import { createPreserveWailsEmbedPlaceholderPlugin } from "./scripts/vite-placeholder.ts";

const frontendDist = path.resolve(import.meta.dirname, "internal/frontend/dist");

export default defineConfig({
  root: "src/renderer",
  base: "/",
  plugins: [
    vue(),
    createPreserveWailsEmbedPlaceholderPlugin(frontendDist),
  ],
  resolve: {
    alias: {
      "@renderer": path.resolve(import.meta.dirname, "src/renderer/src"),
      "@shared": path.resolve(import.meta.dirname, "src/shared"),
    },
  },
  build: {
    outDir: frontendDist,
    emptyOutDir: true,
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            { name: "vue", test: /node_modules[\\/](vue|@vue)[\\/]/, priority: 20 },
            { name: "wails-runtime", test: /node_modules[\\/]@wailsio[\\/]runtime[\\/]/, priority: 20 },
            { name: "ui", test: /node_modules[\\/](reka-ui|@floating-ui|motion-v|framer-motion|motion-dom|motion-utils|@lucide)[\\/]/, priority: 10 },
          ],
        },
      },
    },
  },
  server: {
    host: "127.0.0.1",
    port: 5174,
    strictPort: true,
    fs: {
      allow: [
        path.resolve(import.meta.dirname),
        path.resolve(import.meta.dirname, "../design"),
      ],
    },
  },
});

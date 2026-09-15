import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

import { createPreserveWailsEmbedPlaceholderPlugin } from "./scripts/vite-placeholder.ts";

const frontendDist = path.resolve(import.meta.dirname, "internal/frontend/dist");

export default defineConfig({
  root: "src/renderer",
  base: "/",
  plugins: [
    react(),
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
            { name: "react", test: /node_modules[\\/](react|react-dom|scheduler)[\\/]/, priority: 20 },
            { name: "wails-runtime", test: /node_modules[\\/]@wailsio[\\/]runtime[\\/]/, priority: 20 },
            { name: "fluent-ui", test: /node_modules[\\/]@fluentui[\\/]/, priority: 10 },
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
        path.resolve(import.meta.dirname, "../templates/help.menu/assets/fonts"),
      ],
    },
  },
});

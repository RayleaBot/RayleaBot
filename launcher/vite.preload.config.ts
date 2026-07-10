import path from "node:path";
import { defineConfig } from "vite";

export default defineConfig({
  build: {
    emptyOutDir: true,
    lib: {
      entry: path.resolve(__dirname, "src/preload/index.ts"),
      fileName: () => "index.js",
      formats: ["cjs"],
    },
    minify: false,
    outDir: path.resolve(__dirname, "dist/preload"),
    rolldownOptions: {
      external: ["electron"],
      output: {
        codeSplitting: false,
      },
    },
    sourcemap: false,
  },
});

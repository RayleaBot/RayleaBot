import fs from "node:fs/promises";
import path from "node:path";

export function createPreserveWailsEmbedPlaceholderPlugin(outputDirectory: string) {
  const preservePlaceholder = async () => {
    await fs.mkdir(outputDirectory, { recursive: true });
    await fs.writeFile(path.join(outputDirectory, ".gitkeep"), "\n", "utf8");
  };
  return {
    name: "preserve-wails-embed-placeholder",
    async buildEnd(error?: Error) {
      if (error) {
        await preservePlaceholder();
      }
    },
    async closeBundle() {
      await preservePlaceholder();
    },
  };
}

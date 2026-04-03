import path from "node:path";

export function resolveLauncherBasePath(input: {
  appPath: string;
  executablePath: string;
  isPackaged: boolean;
}) {
  return input.isPackaged ? path.dirname(path.resolve(input.executablePath)) : path.resolve(input.appPath);
}

export function resolveLauncherAssetPaths(appPath: string) {
  const root = path.resolve(appPath);
  return {
    preloadPath: path.join(root, "dist", "preload", "preload", "index.js"),
    rendererPath: path.join(root, "dist", "renderer", "index.html"),
  };
}

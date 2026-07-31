import path from "node:path";

const WINDOWS_LAUNCHER_RUNTIME_DIRECTORY = "launcher";

function isWindowsAbsolutePath(input: string) {
  return /^[a-zA-Z]:[\\/]/.test(input) || input.startsWith("\\\\");
}

function resolveAbsolutePath(input: string) {
  if (path.isAbsolute(input) || isWindowsAbsolutePath(input)) {
    return input;
  }
  return path.resolve(input);
}

function getPathFlavor(input: string) {
  return isWindowsAbsolutePath(input) ? path.win32 : path.posix;
}

export function resolveLauncherBasePath(input: {
  appPath: string;
  executablePath: string;
  isPackaged: boolean;
}) {
  if (!input.isPackaged) {
    return resolveAbsolutePath(input.appPath);
  }

  const executablePath = resolveAbsolutePath(input.executablePath);
  const pathFlavor = getPathFlavor(executablePath);
  const executableDirectory = pathFlavor.dirname(executablePath);
  if (
    pathFlavor === path.win32
    && pathFlavor.basename(executableDirectory).toLowerCase() === WINDOWS_LAUNCHER_RUNTIME_DIRECTORY
  ) {
    return pathFlavor.dirname(executableDirectory);
  }
  return executableDirectory;
}

export function resolveLauncherAssetPaths(appPath: string) {
  const root = resolveAbsolutePath(appPath);
  const pathFlavor = getPathFlavor(root);
  return {
    preloadPath: pathFlavor.join(root, "dist", "preload", "index.js"),
    rendererPath: pathFlavor.join(root, "dist", "renderer", "index.html"),
  };
}

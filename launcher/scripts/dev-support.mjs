import path from "node:path";

export function createDevWaitOnOptions(root) {
  return {
    resources: [
      path.join(root, "dist", "main", "main", "index.js"),
      path.join(root, "dist", "preload", "preload", "index.js"),
      "http-get://127.0.0.1:5174/",
    ],
    timeout: 120000,
  };
}

export function normalizeChildExitCode(code, signal) {
  if (typeof code === "number") {
    return code;
  }

  return signal ? 1 : 0;
}

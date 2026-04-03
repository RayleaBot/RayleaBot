import path from "node:path";

export function createElectronBuilderInvocation(root, env = process.env) {
  return {
    command: process.execPath,
    args: [
      "--disable-warning=DEP0190",
      path.join(root, "node_modules", "electron-builder", "cli.js"),
      "--dir",
    ],
    options: {
      cwd: root,
      env,
      shell: false,
      stdio: ["inherit", "pipe", "pipe"],
      windowsHide: true,
    },
  };
}

import { shell } from "electron";

export const externalOpener = {
  async openUri(uri: string) {
    await shell.openExternal(uri);
  },
  async openDirectory(directoryPath: string) {
    await shell.openPath(directoryPath);
  },
};

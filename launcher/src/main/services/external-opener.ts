import { shell } from "electron";

export const externalOpener = {
  async openUri(uri: string) {
    await shell.openExternal(uri);
  },
  async openDirectory(directoryPath: string) {
    const result = await shell.openPath(directoryPath);
    if (result) {
      throw new Error(result);
    }
  },
};

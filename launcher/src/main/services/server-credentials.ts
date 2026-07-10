import { randomBytes } from "node:crypto";

export class LauncherServerCredentials {
  private setupTokenValue = "";
  private controlTokenValue = "";

  rotate() {
    this.setupTokenValue = randomBytes(32).toString("base64url");
    this.controlTokenValue = randomBytes(32).toString("base64url");
  }

  clear() {
    this.setupTokenValue = "";
    this.controlTokenValue = "";
  }

  get setupToken() {
    return this.setupTokenValue;
  }

  get controlToken() {
    return this.controlTokenValue;
  }
}

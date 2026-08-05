import { randomBytes } from "node:crypto";

const LAUNCHER_CONTROL_TOKEN_ENV = "RAYLEA_LAUNCHER_CONTROL_TOKEN";

export function consumeLauncherControlTokenEnvironment(environment: NodeJS.ProcessEnv = process.env) {
  const controlToken = environment[LAUNCHER_CONTROL_TOKEN_ENV]?.trim() ?? "";
  delete environment[LAUNCHER_CONTROL_TOKEN_ENV];
  return controlToken;
}

export class LauncherServerCredentials {
  private setupTokenValue = "";
  private controlTokenValue: string;

  constructor(initialControlToken = "") {
    this.controlTokenValue = initialControlToken.trim();
  }

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

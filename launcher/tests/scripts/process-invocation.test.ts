import { describe, expect, test } from "vitest";
import fs from "node:fs";
import path from "node:path";
import { createProcessInvocation } from "../../../scripts/process-invocation.mjs";
import { wailsGenerateBindingsArgs, wailsModuleQuery } from "../../scripts/run-go.mjs";

describe("launcher process invocation", () => {
  test("uses the trusted Go executable handed off by the root launcher", () => {
    const goExecutable = String.raw`C:\Program Files\Go\bin\go.exe`;
    expect(createProcessInvocation("go", ["build"], {
      platform: "win32",
      env: {
        PATH: String.raw`C:\node;C:\Windows\System32;C:\Windows`,
        RAYLEA_GO_EXECUTABLE: goExecutable,
      },
      fileExists: (candidate: string) => candidate === goExecutable,
    })).toEqual({ command: goExecutable, args: ["build"] });
  });

  test("finds the standard Windows Go installation outside PATH", () => {
    const goExecutable = String.raw`C:\Program Files\Go\bin\go.exe`;
    expect(createProcessInvocation("go", ["version"], {
      platform: "win32",
      env: {
        PATH: String.raw`C:\node;C:\Windows\System32;C:\Windows`,
        ProgramFiles: String.raw`C:\Program Files`,
      },
      fileExists: (candidate: string) => candidate === goExecutable,
    })).toEqual({ command: goExecutable, args: ["version"] });
  });

  test("rejects an invalid trusted Go executable", () => {
    expect(() => createProcessInvocation("go", ["version"], {
      platform: "win32",
      env: { RAYLEA_GO_EXECUTABLE: String.raw`C:\missing\go.exe` },
      fileExists: () => false,
    })).toThrow(/RAYLEA_GO_EXECUTABLE/);
  });

  test("leaves unrelated native commands unchanged", () => {
    expect(createProcessInvocation("git", ["status"])).toEqual({ command: "git", args: ["status"] });
  });

  test("generates Wails bindings against the GTK 3 Linux variant", () => {
    const linuxArgs = wailsGenerateBindingsArgs("linux");
    const buildFlagsIndex = linuxArgs.indexOf("-f");
    expect(buildFlagsIndex).toBeGreaterThan(-1);
    expect(linuxArgs[buildFlagsIndex + 1]).toBe("-tags gtk3");
    expect(wailsGenerateBindingsArgs("win32")).not.toContain("-tags gtk3");
  });

  test("uses the Wails CLI version declared by go.mod", () => {
    const goModule = fs.readFileSync(path.resolve(import.meta.dirname, "../../go.mod"), "utf8");
    const version = goModule.match(/^\s*github\.com\/wailsapp\/wails\/v3\s+(v\S+)\s*$/m)?.[1];
    const packageMetadata = JSON.parse(
      fs.readFileSync(path.resolve(import.meta.dirname, "../../package.json"), "utf8"),
    );
    expect(version).toBeTruthy();
    expect(wailsModuleQuery).toBe(`github.com/wailsapp/wails/v3@${version}`);
    expect(wailsGenerateBindingsArgs("win32").slice(0, 2)).toEqual(["generate", "bindings"]);
    expect(packageMetadata.dependencies["@wailsio/runtime"]).toBe(version?.slice(1));
  });
});

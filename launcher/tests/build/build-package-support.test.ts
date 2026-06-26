import path from "node:path";
import { describe, expect, test } from "vitest";
import { createElectronBuilderInvocation } from "../../scripts/build-package-support.mjs";

describe("build-package support", () => {
  test("invokes electron-builder through node without shell-based argument concatenation", () => {
    const root = path.resolve(import.meta.dirname, "..", "..");
    const invocation = createElectronBuilderInvocation(root, {
      PATH: process.env.PATH ?? "",
      CUSTOM_TEST_ENV: "fixture",
    });

    expect(invocation.command).toBe(process.execPath);
    expect(invocation.args[0]).toBe("--disable-warning=DEP0190");
    expect(invocation.args.at(-1)).toBe("--dir");
    expect(invocation.args.some((item) => item.endsWith(path.join("electron-builder", "cli.js")))).toBe(true);
    expect(invocation.options.shell).not.toBe(true);
    expect(invocation.options.env.CUSTOM_TEST_ENV).toBe("fixture");
    expect(invocation.options.env.PATH.split(path.delimiter)[0]).toContain("rayleabot-pnpm-");
    expect(typeof invocation.cleanup).toBe("function");
    invocation.cleanup();
  });
});

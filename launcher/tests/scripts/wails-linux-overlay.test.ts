import { describe, expect, test } from "vitest";
import { patchLinuxTraySource } from "../../scripts/wails-linux-overlay.mjs";
import { prepareLauncherGoArgs } from "../../scripts/run-go.mjs";

describe("Wails Linux compatibility patch guards", () => {
  test("requires review when the frozen Wails version changes", () => {
    expect(() => patchLinuxTraySource("", "v3.0.0-beta.10")).toThrow("Review the Linux tray event patch");
  });

  test("refuses changed or ambiguous upstream event handlers", () => {
    const changed = '\tcase "opened":\n\t\tif s.parent.onMenuOpen != nil {';
    const old = '\tcase "opened":\n\t\tif s.parent.clickHandler != nil {\n\t\t\ts.parent.clickHandler()\n\t\t}\n\t\tif s.parent.onMenuOpen != nil {';
    for (const source of [changed, old + old]) {
      expect(() => patchLinuxTraySource(source, "v3.0.0-beta.9")).toThrow("no longer matches");
    }
  });

  test.each(["win32", "windows", "darwin"])("leaves %s commands intact", async (targetOS) => {
    const args = ["build", "-tags", "production", "."];
    expect(await prepareLauncherGoArgs(args, targetOS)).toBe(args);
  });

  test("does not attach build flags to module commands", async () => {
    const args = ["mod", "download"];
    expect(await prepareLauncherGoArgs(args, "linux")).toBe(args);
  });
});

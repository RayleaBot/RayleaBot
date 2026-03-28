import { describe, expect, test } from "vitest";
import viteConfig from "../../vite.config";

describe("vite renderer packaging config", () => {
  test("uses relative asset base for packaged file:// renderer loads", () => {
    expect(viteConfig.base).toBe("./");
  });
});
